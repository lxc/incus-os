import os
import subprocess

def _check_deps():
    # Expand PATH to include /usr/sbin/
    os.environ["PATH"] = os.environ["PATH"] + ":/usr/sbin"

    # Check that expected commands are available before running any tests
    for cmd in ["curl", "gdisk", "go", "mkfs.vfat", "mcopy", "mdeltree", "mkisofs", "openssl", "truncate"]:
        try:
            subprocess.run(["which", cmd], capture_output=True, check=True)
        except:
            print("Missing command: " + cmd)
            exit(1)
