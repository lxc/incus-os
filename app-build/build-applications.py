#!/usr/bin/python3

import json
import os
import platform
import subprocess

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
    ["base", ["incus-osd", "kpx", "tailscale"]],
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

    # Apply version substitutions before doing anything else
    for values in applications[artifact]["build_targets"]:
        for i in range(0, len(values)):
            values[i] = values[i].replace("@VERSION@", version)
    for values in applications[artifact]["install_targets"]:
        values[0] = values[0].replace("@VERSION@", version)
        values[1] = values[1].replace("@VERSION@", version)
        values[0] = values[0].replace("@ARCH@", ARCH)
        values[1] = values[1].replace("@ARCH@", ARCH)
    for values in applications[artifact].get("rename_targets", []):
        values[0] = values[0].replace("@VERSION@", version)
        values[1] = values[1].replace("@VERSION@", version)

    targets = applications[artifact]["build_targets"]

    # Clone/update the git repo
    if os.path.isdir(artifact):
        subprocess.run(["git", "reset", "--hard"], cwd=artifact, check=True)
        if version == "main":
            subprocess.run(["git", "pull"], cwd=artifact, check=True)
        else:
            subprocess.run(["git", "fetch", "--depth", "1", "origin", "v"+version+":refs/tags/v"+version], cwd=artifact, check=True)
            subprocess.run(["git", "checkout", "v"+version], cwd=artifact, check=True)
    else:
        if version == "main":
            subprocess.run(["git", "clone", repo, artifact, "--depth", "1", "-b", version], check=True)
        else:
            subprocess.run(["git", "clone", repo, artifact, "--depth", "1", "-b", "v"+version], check=True)

    # Apply any patches
    for patch in applications[artifact].get("patches", []):
        subprocess.run(["patch", "-p1", "-i", patch], cwd=artifact, check=True)

    # Build the targets
    for target in targets:
        subprocess.run(["go", "build", *target], cwd=artifact, check=True)

    if applications[artifact].get("build_ui"):
        env = os.environ.copy()
        env["YARN_ENABLE_HARDENED_MODE"] = "0"
        env["YARN_ENABLE_IMMUTABLE_INSTALLS"] = "false"
        subprocess.run(["yarnpkg", "install"], cwd=os.path.join(artifact, "ui"), env=env, check=True)
        subprocess.run(["yarnpkg", "build"], cwd=os.path.join(artifact, "ui"), env=env, check=True)

    # Symlink targets
    for link in applications[artifact].get("link_targets", []):
        subprocess.run(["rm", "-f", link[1]], cwd=artifact, check=True)
        subprocess.run(["ln", "-s", *link], cwd=artifact, check=True)

    # Generate the application's manifest
    create_application_manifest(artifact, version)

    # Rename any files or directories after everything is done
    for rename in applications[artifact].get("rename_targets", []):
        subprocess.run(["rm", "-rf", rename[1]], cwd=artifact, check=True)
        subprocess.run(["mv", rename[0], rename[1]], cwd=artifact, check=True)

def create_application_manifest(artifact, version):
    # If building from main, set the version to be the current commit
    if version == "main":
        version = subprocess.run(["git", "rev-parse", "HEAD"], cwd=artifact, capture_output=True, check=True).stdout.strip().decode("utf-8")

    manifest = {
        "name": artifact,
        "version": version,
        "repo": applications[artifact]["repo"],
        "installed_artifacts": [],
        "go_compiler": subprocess.run(["go", "version"], capture_output=True, check=True).stdout.strip().decode("utf-8"),
        "go_packages": [],
    }

    for target in applications[artifact]["install_targets"]:
        manifest["installed_artifacts"].append(os.path.join("/", target[1], os.path.basename(target[0])))

    direct_deps = subprocess.run(["go", "list", "-mod=mod", "-m", "-f", "{{if not (or .Indirect .Main)}}{{.Path}} {{.Version}}{{end}}", "all"], cwd=artifact, capture_output=True, check=True).stdout.strip().decode("utf-8")
    for line in direct_deps.split("\n"):
        parts = line.split(" ")
        manifest["go_packages"].append({
            "type": "go",
            "name": parts[0],
            "version": parts[1],
            "direct": True
        })

    indirect_deps = subprocess.run(["go", "list", "-mod=mod", "-m", "-f", "{{if .Indirect}}{{.Path}} {{.Version}}{{end}}", "all"], cwd=artifact, capture_output=True, check=True).stdout.strip().decode("utf-8")
    for line in indirect_deps.split("\n"):
        parts = line.split(" ")
        manifest["go_packages"].append({
            "type": "go",
            "name": parts[0],
            "version": parts[1],
            "direct": False
        })

    if applications[artifact].get("build_ui"):
        manifest["yarn_version"] = subprocess.run(["yarnpkg", "--version"], capture_output=True, check=True).stdout.strip().decode("utf-8")
        manifest["yarn_packages"] = []

        yarn_info = subprocess.run(["yarnpkg", "info", "--json", "--name-only"], cwd=os.path.join(artifact, "ui"), capture_output=True, check=True).stdout.strip().decode("utf-8")
        for line in yarn_info.split("\n"):
            parts = line.replace("\"", "").rsplit("@", 1)
            manifest["yarn_packages"].append({
                "type": "node",
                "name": parts[0],
                "version": parts[1],
                "direct": True # FIXME -- need to figure out how to determine if a node dependency is direct or indirect
            })

    with open(artifact+".json", "w") as f:
        json.dump(manifest, f)

def install(image, artifact):
    base_path = "../mkosi.images/"+image+"/mkosi.extra"

    for target in applications[artifact]["clean_targets"]:
        # Clean any previously installed files
        subprocess.run(["rm", "-rf", os.path.join(base_path, target)], check=True)

    for target in applications[artifact]["install_targets"]:
        if os.path.isfile(os.path.join(artifact, target[0])):
            # Strip the binary
            subprocess.run(["strip", target[0]], cwd=artifact, check=True)

        # Copy the target into the mkosi image filesystem
        subprocess.run(["mkdir", "-p", os.path.join(base_path, target[1])], check=True)
        subprocess.run(["cp", "-r", os.path.join(artifact, target[0]), os.path.join(base_path, target[1])], check=True)

def create_image_manifest(image, applications):
    manifest = []

    for app in applications:
        with open(app+".json", "r") as f:
            manifest.append(json.load(f))

    with open(image+".json", "w") as f:
        json.dump(manifest, f)


if __name__ == "__main__":
    for app in applications:
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
