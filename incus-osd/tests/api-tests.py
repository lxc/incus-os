#!/usr/bin/python3

from common import _check_deps, _download_images, _run_tests
from incusos_tests import IncusOSTests

_check_deps()

prior_image_img, current_image_img, current_image_iso = _download_images()

tests = IncusOSTests(prior_image_img, current_image_img, current_image_iso)

_run_tests(tests)
