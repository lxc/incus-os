#!/usr/bin/python3

import json
import subprocess

applications = {}
with open("applications.json", "r") as f:
    applications = json.load(f)


for app in applications:
    if app == "incus-osd":
        continue

    if applications[app]["version"] == "main":
        continue

    version = subprocess.run(["sh", "-c", """ git ls-remote --sort="v:refname" --tags "%s" 2>/dev/null | grep -v '{}' | grep -v \\\\.99 | grep -Ev 'rc|beta|alpha|pre|vUDK' | tail -1 | sed "s#.*refs/tags/##g" """ % applications[app]["repo"]], capture_output=True, check=True).stdout.strip().decode("utf-8")

    if applications[app]["version"] != version:
        print(app + " updated to version " + version)
        applications[app]["version"] = version

with open("applications.json", "w") as f:
    json.dump(applications, f, indent="    ")
    f.write("\n")
