import io
import os
import random
import shutil
import string
import tarfile

def _get_random_string():
    return "".join(random.choices(string.ascii_letters + string.digits, k=10))

def _prepare_test_image(image, seed):
    ext = ".img" if image.endswith(".img") else ".iso"

    parent_dir = os.path.dirname(image)
    basename = os.path.basename(image)
    test_image = os.path.join(parent_dir, basename.replace(ext, "_"+_get_random_string()+ext))

    # Create a copy of the install image.
    shutil.copy(image, test_image)

    # Inject seed data, if any.
    with open(test_image, "rb+") as f:
        f.seek(4196352*512)

        with tarfile.open(mode="w", fileobj=f) as tar:
            for filename, contents in seed.items():
                raw = contents.encode("utf-8")
                buf = io.BytesIO(raw)
                ti = tarfile.TarInfo(name=filename)
                ti.size = len(raw)

                tar.addfile(ti, buf)

    # Return the path of the customized install image and IncusOS version.
    return test_image, basename.replace("IncusOS_", "").replace(ext, "")
