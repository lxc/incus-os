#!/usr/bin/python3

from common import _check_deps, _download_images, _run_tests
from incusos_tests import CertificateTests

_check_deps()

prior_image_img, _, _ = _download_images(require_prior_release_stable=False)

tests = CertificateTests(prior_image_img)

# Run certificate end-to-end tests one at a time. This is because tests
# may temporarily cause update metadata to be invalid, and we don't want
# a race condition with another test running concurrently and then
# randomly failing.
_run_tests(tests, max_workers=1)
