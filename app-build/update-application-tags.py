#!/usr/bin/python3

import json
import subprocess
import urllib.request


def latest_github_release(repo):
    # The latest release page redirects to the tag and skips pre-releases.
    url = repo.removesuffix(".git") + "/releases/latest"
    with urllib.request.urlopen(url) as resp:
        final_url = resp.geturl()

    if "/releases/tag/" not in final_url:
        return ""

    return final_url.split("/releases/tag/")[1]


applications = {}
with open("applications.json", "r") as f:
    applications = json.load(f)


for app in applications:
    if app == "incus-osd":
        continue

    if applications[app]["version"] == "main":
        continue

    if applications[app].get("github_release", False):
        version = latest_github_release(applications[app]["repo"])
    else:
        version = subprocess.run(["sh", "-c", """ git ls-remote --sort="v:refname" --tags "%s" 2>/dev/null | grep -v '{}' | grep -v \\\\.99 | grep -Ev 'rc|beta|alpha|pre|vUDK|pr-' | tail -1 | sed "s#.*refs/tags/##g" """ % applications[app]["repo"]], capture_output=True, check=True).stdout.strip().decode("utf-8")

    if version == "":
        # No suitable tag, pin to the current HEAD commit.
        version = subprocess.run(["git", "ls-remote", applications[app]["repo"], "HEAD"], capture_output=True, check=True).stdout.strip().decode("utf-8").split("\t")[0]

    if applications[app]["version"] != version and version.startswith(applications[app].get("version_prefix", "")):
        print(app + " updated to version " + version)
        applications[app]["version"] = version

with open("applications.json", "w") as f:
    json.dump(applications, f, indent="    ")
    f.write("\n")
