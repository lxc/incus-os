#!/usr/bin/python3

import concurrent.futures
import gzip
import json
import os
import requests
import shutil
import urllib.request

from incusos_tests import IncusOSTests

current_version = ""
prior_version = ""
urls = []

# Fetch the current and immediate prior release information
with urllib.request.urlopen("https://images.linuxcontainers.org/os/index.json") as url:
    versions = json.loads(url.read().decode())

    if len(versions["updates"]) < 2:
        raise Exception("need at least two published versions")

    current_version = versions["updates"][0]["version"]
    prior_version = versions["updates"][1]["version"]

    for file in versions["updates"][0]["files"]:
        if file["architecture"] == "x86_64":
            if file["type"] == "image-raw" or file["type"] == "image-iso":
                urls.append("https://images." + versions["updates"][0]["origin"] + "/os" + versions["updates"][0]["url"] + "/" + file["filename"])

    for file in versions["updates"][1]["files"]:
        if file["architecture"] == "x86_64":
            if file["type"] == "image-raw":
                urls.append("https://images." + versions["updates"][1]["origin"] + "/os" + versions["updates"][1]["url"] + "/" + file["filename"])

# Download images if needed
for url in urls:
    basename = os.path.basename(url)

    if not os.path.exists(basename.replace(".gz", "")):
        print("Downloading IncusOS image " + basename, flush=True)

        with requests.get(url, stream=True) as r:
            with open(basename, "wb") as f:
                shutil.copyfileobj(r.raw, f)

        with gzip.open(basename, "rb") as f_in:
            with open(basename.replace(".gz", ""), "wb") as f_out:
                shutil.copyfileobj(f_in, f_out)

        os.remove(basename)

prior_image_img = os.path.join(os.getcwd(), "IncusOS_" + prior_version + ".img")
current_image_img = os.path.join(os.getcwd(), "IncusOS_" + current_version + ".img")
current_image_iso = os.path.join(os.getcwd(), "IncusOS_" + current_version + ".iso")

num_pass = 0
num_fail = 0

# Run the tests
with concurrent.futures.ThreadPoolExecutor(max_workers=1) as executor:
    tests = IncusOSTests(prior_image_img, current_image_img, current_image_iso)
    futures = {executor.submit(fn, image): name for name,fn,image in tests.GetTests()}

    print("Running %d tests...\n" % len(futures), flush=True)

    for future in concurrent.futures.as_completed(futures):
        name = futures[future]

        try:
            data = future.result()
        except Exception as e:
            num_fail += 1
            print("FAIL: %s: %s" % (name, e), flush=True)
        else:
            num_pass += 1
            print("PASS: %s" % name, flush=True)

print("\nDone with tests: %d/%d passed." % (num_pass, num_fail+num_pass), flush=True)

if num_fail > 0:
    exit(1)
