#!/usr/bin/python3

import json
import subprocess
import urllib.request

applications = {}
with open("applications.json", "r") as f:
    applications = json.load(f)


# The NVIDIA driver doesn't come from a git repository and must match the version of its kernel modules
NVIDIA_DRIVER = "nvidia-driver"
NVIDIA_KERNEL_MODULES = "nvidia-open-gpu-kernel-modules"
NVIDIA_RELEASE_FEED = "https://docs.nvidia.com/datacenter/tesla/drivers/releases.json"


def nvidia_driver_version():
    with urllib.request.urlopen(NVIDIA_RELEASE_FEED) as resp:
        feed = json.load(resp)

    # Use the most recent release from the newest production or long term support branch
    branches = [branch for branch in feed if feed[branch]["type"] in ("production branch", "lts branch")]
    releases = [release for release in feed[max(branches, key=int)]["driver_info"] if {"x86_64", "aarch64"} <= set(release["architectures"])]

    return max(releases, key=lambda release: [int(part) for part in release["release_version"].split(".")])["release_version"]


def update_nvidia_versions():
    version = nvidia_driver_version()

    if applications[NVIDIA_DRIVER]["version"] == version:
        return

    # Wait for the matching kernel modules to be tagged
    tag = subprocess.run(["git", "ls-remote", "--tags", applications[NVIDIA_KERNEL_MODULES]["repo"], "refs/tags/" + version], capture_output=True, check=True).stdout.strip()
    if tag == b"":
        print(NVIDIA_DRIVER + " held back from version " + version)
        return

    for app in (NVIDIA_DRIVER, NVIDIA_KERNEL_MODULES):
        print(app + " updated to version " + version)
        applications[app]["version"] = version


for app in applications:
    if app == "incus-osd":
        continue

    if applications[app]["version"] == "main":
        continue

    # The NVIDIA applications are updated together from the NVIDIA release feed
    if app in (NVIDIA_DRIVER, NVIDIA_KERNEL_MODULES):
        continue

    version = subprocess.run(["sh", "-c", """ git ls-remote --sort="v:refname" --tags "%s" 2>/dev/null | grep -v '{}' | grep -v \\\\.99 | grep -Ev 'rc|beta|alpha|pre|vUDK|pr-' | tail -1 | sed "s#.*refs/tags/##g" """ % applications[app]["repo"]], capture_output=True, check=True).stdout.strip().decode("utf-8")

    if version == "":
        # No suitable tag, pin to the current HEAD commit.
        version = subprocess.run(["git", "ls-remote", applications[app]["repo"], "HEAD"], capture_output=True, check=True).stdout.strip().decode("utf-8").split("\t")[0]

    if applications[app]["version"] != version and version.startswith(applications[app].get("version_prefix", "")):
        print(app + " updated to version " + version)
        applications[app]["version"] = version

update_nvidia_versions()

with open("applications.json", "w") as f:
    json.dump(applications, f, indent="    ")
    f.write("\n")
