#!/usr/bin/python3

import gzip
import json
import os
import os.path
import platform
import requests
import subprocess
import tarfile

# Detect architecture string for tofu providers
ARCH = platform.machine()
if ARCH == "x86_64":
    ARCH = "amd64"
elif ARCH == "aarch64":
    ARCH = "arm64"
else:
    raise("Unsupported architecture " + ARCH)

applications = {}
with open("applications.json", "r") as f:
    applications = json.load(f)

images = [
    ["base", ["incus-osd", "kpx", "linux-firmware-base", "netbird", "tailscale"]],
    ["gpu-support", [
        "linux-firmware-gpu"]
    ],
    ["migration-manager", [
        "lego",
        "migration-manager"]
    ],
    ["operations-center", [
        "lego",
        "opentofu",
        "operations-center",
        "terraform-provider-incus",
        "terraform-provider-null",
        "terraform-provider-random",
        "terraform-provider-time"]
    ]
]

def build(artifact):
    version = applications[artifact]["version"]
    repo = applications[artifact]["repo"]
    directory = applications[artifact].get("directory", artifact)

    # Apply version substitutions before doing anything else
    for values in applications[artifact]["build_targets"]:
        for i in range(0, len(values)):
            values[i] = values[i].replace("@TAG@", version)
            values[i] = values[i].replace("@VERSION@", version.removeprefix("v"))
    for values in applications[artifact]["install_targets"]:
        values[0] = values[0].replace("@TAG@", version)
        values[1] = values[1].replace("@TAG@", version)
        values[0] = values[0].replace("@VERSION@", version.removeprefix("v"))
        values[1] = values[1].replace("@VERSION@", version.removeprefix("v"))
        values[0] = values[0].replace("@ARCH@", ARCH)
        values[1] = values[1].replace("@ARCH@", ARCH)
    for values in applications[artifact].get("rename_targets", []):
        values[0] = values[0].replace("@TAG@", version)
        values[1] = values[1].replace("@TAG@", version)
        values[0] = values[0].replace("@VERSION@", version.removeprefix("v"))
        values[1] = values[1].replace("@VERSION@", version.removeprefix("v"))

    targets = applications[artifact]["build_targets"]

    # Clone/update the git repo
    if os.path.isdir(directory):
        subprocess.run(["git", "reset", "--hard"], cwd=directory, check=True)
        if version == "main":
            subprocess.run(["git", "pull"], cwd=directory, check=True)
        else:
            subprocess.run(["git", "fetch", "--depth", "1", "origin", version+":refs/tags/"+version], cwd=directory, check=True)
            subprocess.run(["git", "checkout", version], cwd=directory, check=True)
    else:
        if version == "main":
            subprocess.run(["git", "clone", repo, directory, "--depth", "1", "-b", version], check=True)
        else:
            subprocess.run(["git", "clone", repo, directory, "--depth", "1", "-b", version], check=True)

    # Handle git submodules
    if applications[artifact].get("submodules", False):
        subprocess.run(["git", "submodule", "init"], cwd=directory, check=True)
        subprocess.run(["git", "submodule", "update"], cwd=directory, check=True)

    # Apply any patches
    for patch in applications[artifact].get("patches", []):
        subprocess.run(["patch", "-p1", "-i", patch], cwd=directory, check=True)

    # Build the targets
    for target in targets:
        subprocess.run(["go", "build", *target], cwd=directory, check=True)

    if applications[artifact].get("download_ui"):
        subprocess.run(["rm", "-rf", "ui"], cwd=directory, check=True)
        download_extract_github_asset("https://github.com/FuturFusion/" + artifact + "/releases/download/" + version + "/ui.tar.gz", os.path.join(directory, "ui"))

    # Conditionally download the Migration Manager worker image if performing an x86_64 build
    if artifact == "migration-manager" and ARCH == "amd64":
        subprocess.run(["rm", "-f", "worker-x86_64.img"], cwd=directory, check=True)
        download_extract_github_asset("https://github.com/FuturFusion/migration-manager/releases/download/" + version + "/migration-manager-worker.img.gz", directory)
        subprocess.run(["mv", "migration-manager-worker.img", "worker-x86_64.img"], cwd=directory, check=True)

    # Symlink targets
    for link in applications[artifact].get("link_targets", []):
        subprocess.run(["rm", "-f", link[1]], cwd=directory, check=True)
        subprocess.run(["ln", "-s", *link], cwd=directory, check=True)

    # Generate the application's manifest
    create_application_manifest(artifact, version)

    # Rename any files or directories after everything is done
    for rename in applications[artifact].get("rename_targets", []):
        subprocess.run(["rm", "-rf", rename[1]], cwd=directory, check=True)
        subprocess.run(["mv", rename[0], rename[1]], cwd=directory, check=True)

def create_application_manifest(artifact, version):
    directory = applications[artifact].get("directory", artifact)

    # If building from main, set the version to be the current commit
    if version == "main":
        version = subprocess.run(["git", "rev-parse", "HEAD"], cwd=directory, capture_output=True, check=True).stdout.strip().decode("utf-8")

        # Check if a tag points to this commit, and if so, prefer that over the commit
        tags = subprocess.run(["git", "tag", "--points-at", version], cwd=directory, capture_output=True, check=True).stdout.strip().decode("utf-8").split("\n")
        if len(tags) > 0 and tags[0] != "":
            version = tags[0]

    manifest = {
        "name": artifact,
        "version": version,
        "repo": applications[artifact]["repo"],
        "installed_artifacts": [],
    }

    for target in applications[artifact]["install_targets"]:
        manifest["installed_artifacts"].append(os.path.join("/", target[1], os.path.basename(target[0])))

    if os.path.exists("go.mod"):
        manifest["go_compiler"] = subprocess.run(["go", "version"], capture_output=True, check=True).stdout.strip().decode("utf-8")
        manifest["go_packages"] = []

        direct_deps = subprocess.run(["go", "list", "-mod=mod", "-m", "-f", "{{if not (or .Indirect .Main)}}{{.Path}} {{.Version}}{{end}}", "all"], cwd=directory, capture_output=True, check=True).stdout.strip().decode("utf-8")
        for line in direct_deps.split("\n"):
            parts = line.split(" ")
            manifest["go_packages"].append({
                "type": "go",
                "name": parts[0],
                "version": parts[1],
                "direct": True
            })

        indirect_deps = subprocess.run(["go", "list", "-mod=mod", "-m", "-f", "{{if .Indirect}}{{.Path}} {{.Version}}{{end}}", "all"], cwd=directory, capture_output=True, check=True).stdout.strip().decode("utf-8")
        for line in indirect_deps.split("\n"):
            parts = line.split(" ")
            manifest["go_packages"].append({
                "type": "go",
                "name": parts[0],
                "version": parts[1],
                "direct": False
            })

    with open(artifact+".json", "w") as f:
        json.dump(manifest, f)

def install(image, artifact):
    base_path = "../mkosi.images/"+image+"/mkosi.extra"
    directory = applications[artifact].get("directory", artifact)

    for target in applications[artifact]["clean_targets"]:
        # Clean any previously installed files
        subprocess.run(["rm", "-rf", os.path.join(base_path, target)], check=True)

    for target in applications[artifact]["install_targets"]:
        if os.path.isfile(os.path.join(directory, target[0])):
            # Strip the binary
            subprocess.run(["strip", target[0]], cwd=directory, check=True)

        # Copy the target into the mkosi image filesystem
        subprocess.run(["mkdir", "-p", os.path.join(base_path, target[1])], check=True)
        subprocess.run(["cp", "-r", os.path.join(directory, target[0]), os.path.join(base_path, target[1])], check=True)

    # Conditionally install the Migration Manager worker image if performing an x86_64 build
    if artifact == "migration-manager" and ARCH == "amd64":
        subprocess.run(["mkdir", "-p", os.path.join(base_path, "usr/share/migration-manager/images/")], check=True)
        subprocess.run(["cp", "migration-manager/worker-x86_64.img", os.path.join(base_path, "usr/share/migration-manager/images/")], check=True)

def create_image_manifest(image, applications):
    manifest = []

    for app in applications:
        with open(app+".json", "r") as f:
            manifest.append(json.load(f))

    with open(image+".json", "w") as f:
        json.dump(manifest, f)

def download_extract_github_asset(asset_url, directory):
    req = requests.get(asset_url, stream=True)
    if asset_url.endswith(".tar.gz"):
        tf = tarfile.open(fileobj=req.raw, mode="r|gz")
        tf.extractall(path=directory)
    elif asset_url.endswith(".gz"):
        with gzip.open(req.raw) as gz:
            with open(os.path.join(directory, os.path.basename(asset_url).removesuffix(".gz")), "wb") as f:
                while True:
                    chunk = gz.read(100*1024*1024)
                    if not chunk:
                        break

                    f.write(chunk)
    else:
        raise Exception("Unsupported asset: " + asset_url)


if __name__ == "__main__":
    for app in applications:
        skip = False
        for requirement in applications[app].get("requires", []):
            if not os.path.exists(requirement):
                print("Skipping " + app + " due to missing requirement")
                skip = True
                break
        if skip:
            continue

        if app == "incus-osd":
            # incus-osd is already built, so we only need to generate its manifest
            create_application_manifest("incus-osd", "main")
        else:
            print("Building " + app)
            build(app)

    for image, apps in images:
        for app in apps:
            print("Installing " + app + " for image " + image)
            install(image, app)

        print("Creating manifest for " + image)
        create_image_manifest(image, apps)
