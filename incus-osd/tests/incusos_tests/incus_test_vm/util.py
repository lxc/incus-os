import io
import os
import random
import shutil
import string
import subprocess
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
    if seed is not None:
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

def _create_user_media(f, d, media_type, media_size, media_label):
    if media_type == "img":
        f.truncate(media_size)
        subprocess.run(["/sbin/sgdisk", "-n", "1", "-c", "1:" + media_label, f.name], capture_output=True, check=True)
        subprocess.run(["/sbin/mkfs.vfat", "-S", "512", "--offset=2048", f.name], capture_output=True, check=True)

        for entry in os.scandir(d):
            subprocess.run(["mcopy", "-s", "-i", f.name+"@@1048576", entry.path, "::" + entry.path.removeprefix(d)], capture_output=True, check=True)
    else:
        subprocess.run(["mkisofs", "-V", media_label, "-joliet-long", "-rock", "-o", f.name, d], capture_output=True, check=True)

def _extract_secureboot_keys(image, directory):
    subprocess.run(["mcopy", "-i", image+"@@1048576", "::loader/keys/auto/*", directory], capture_output=True, check=True)

def _remove_secureboot_keys(image):
    subprocess.run(["mdeltree", "-i", image+"@@1048576", "::loader/keys/auto/"], capture_output=True, check=True)
