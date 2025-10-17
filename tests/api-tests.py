#!/usr/bin/python3

import concurrent.futures
import os

from incus_test_vm import tests

old_image_img = os.path.join(os.getcwd(), "IncusOS_202510232320.img")
image_img = os.path.join(os.getcwd(), "IncusOS_202510240220.img")
image_iso = os.path.join(os.getcwd(), "IncusOS_202510240220.iso")

num_pass = 0
num_fail = 0

with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
    futures = {executor.submit(fn, image): name for name,fn,image in tests.GetTests(image_img, image_iso, old_image_img)}

    print("Running %d tests..." % len(futures))

    for future in concurrent.futures.as_completed(futures):
        name = futures[future]

        try:
            data = future.result()
        except Exception as e:
            num_fail += 1
            print("FAIL: %s: %s" % (name, e))
        else:
            num_pass += 1
            print("PASS: %s" % name)

print("\nDone with tests: %d/%d passed." % (num_pass, num_fail+num_pass))
