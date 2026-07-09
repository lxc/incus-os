#!/usr/bin/python3

import concurrent.futures
import gzip
import json
import os
import requests
import shutil
import urllib.request

from common import _check_deps
from incusos_tests import IncusOSTests
from incusos_tests.incus_test_vm import IncusOSException

os_name = ""
current_release = None
prior_stable_release = None
urls = []

_check_deps()

IMAGES_SERVER = os.getenv("IMAGES_SERVER", "https://images.linuxcontainers.org")

# Fetch the current and prior stable release information
with urllib.request.urlopen(IMAGES_SERVER + "/os/index.json") as url:
    versions = json.loads(url.read().decode())

    current_stable_release_version = ""

    for update in versions["updates"]:
        if current_release is None:
            current_release = update

        if "stable" in update["channels"]:
            if current_stable_release_version == "":
                current_stable_release_version = update["version"]
            else:
                prior_stable_release = update
                break

    if prior_stable_release is None:
        raise IncusOSException("need at least two published stable releases")

    for file in current_release["files"]:
        if file["architecture"] == "x86_64":
            if file["type"] == "image-raw" or file["type"] == "image-iso":
                urls.append(IMAGES_SERVER + "/os" + current_release["url"] + "/" + file["filename"])

    for file in prior_stable_release["files"]:
        if file["architecture"] == "x86_64":
            if file["type"] == "image-raw":
                urls.append(IMAGES_SERVER + "/os" + prior_stable_release["url"] + "/" + file["filename"])

# Download images if needed
for url in urls:
    basename = os.path.basename(url)

    if os_name == "":
        os_name = basename.split("_")[0]

    if not os.path.exists(basename.replace(".gz", "")):
        print("Downloading " + os_name + " image " + basename, flush=True)

        with requests.get(url, stream=True) as r:
            with open(basename, "wb") as f:
                shutil.copyfileobj(r.raw, f)

        with gzip.open(basename, "rb") as f_in:
            with open(basename.replace(".gz", ""), "wb") as f_out:
                shutil.copyfileobj(f_in, f_out)

        os.remove(basename)

prior_image_img = os.path.join(os.getcwd(), os_name + "_" + prior_stable_release["version"] + ".img")
current_image_img = os.path.join(os.getcwd(), os_name + "_" + current_release["version"] + ".img")
current_image_iso = os.path.join(os.getcwd(), os_name + "_" + current_release["version"] + ".iso")

num_pass = 0
num_fail = 0

# Run the tests
with concurrent.futures.ThreadPoolExecutor(max_workers=3) as executor:
    tests = IncusOSTests(prior_image_img, current_image_img, current_image_iso)
    futures = {executor.submit(fn, image): name for name,fn,image in tests.GetTests()}

    print("Running %d tests...\n" % len(futures), flush=True)

    for future in concurrent.futures.as_completed(futures):
        name = futures[future]

        try:
            data = future.result()
        except IncusOSException as e:
            num_fail += 1
            print("FAIL: %s: %s" % (name, e.args[0]), flush=True)

            if len(e.args) == 2:
                print("          journalctl entries:", flush=True)
                for line in e.args[1]:
                    if line != "":
                        print("          %s" % line, flush=True)
        except Exception as e:
            num_fail += 1
            print("FAIL: %s: %s" % (name, e), flush=True)
        else:
            num_pass += 1
            print("PASS: %s" % name, flush=True)

print("\nDone with tests: %d/%d passed." % (num_pass, num_fail+num_pass), flush=True)

if num_fail > 0:
    exit(1)
