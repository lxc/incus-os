import io
import json
import os
import random
import shutil
import string
import subprocess
import tarfile
import tempfile
import urllib.request

def _get_random_string():
    return "".join(random.choices(string.ascii_letters + string.digits, k=10))

def _prepare_test_image(image, seed):
    ext = ".img" if image.endswith(".img") else ".iso"

    parent_dir = os.path.dirname(image)
    basename = os.path.basename(image)
    test_image = os.path.join(parent_dir, basename.replace(ext, "_"+_get_random_string()+ext))

    # Create a copy of the install image.
    shutil.copy(image, test_image)

    client_cert_name = None
    client_cert_pub = None

    # Generate a temporary client TLS certificate
    with tempfile.NamedTemporaryFile(dir=os.getcwd(), delete=False, delete_on_close=True) as cert_file:
        client_cert_name = cert_file.name

        output = subprocess.run(["openssl", "req", "-new", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:secp384r1", "-x509", "-addext", "extendedKeyUsage = clientAuth", "-nodes", "-days", "365", "-subj", "/OU=Linux Containers/CN=test@localhost", "-out", client_cert_name, "-keyout", "-"], capture_output=True, check=True)

        cert_file.seek(0)
        client_cert_pub = cert_file.read()
        client_cert_key = output.stdout

        cert_file.seek(0)
        cert_file.write(client_cert_key)
        cert_file.write(client_cert_pub)

    if seed is None:
        seed = {}

    # Set incus client TLS certificate
    if "incus.json" not in seed:
        seed["incus.json"] = "{}"

    incus_json = json.loads(seed["incus.json"])

    if "apply_defaults" not in incus_json:
        incus_json["apply_defaults"] = True

    if "preseed" not in incus_json:
        incus_json["preseed"] = {}

    if "certificates" not in incus_json["preseed"]:
        incus_json["preseed"]["certificates"] = []

    incus_json["preseed"]["certificates"].append({
        "type": "client",
        "certificate": client_cert_pub.decode("UTF-8")
    })

    seed["incus.json"] = json.dumps(incus_json)

    # Set migration manager client TLS certificate
    if "migration-manager.json" not in seed:
        seed["migration-manager.json"] = "{}"

    mm_json = json.loads(seed["migration-manager.json"])

    if "trusted_client_certificates" not in mm_json:
        mm_json["trusted_client_certificates"] = []

    mm_json["trusted_client_certificates"].append(client_cert_pub.decode("UTF-8"))

    seed["migration-manager.json"] = json.dumps(mm_json)

    # Set operations center client TLS certificate
    if "operations-center.json" not in seed:
        seed["operations-center.json"] = "{}"

    oc_json = json.loads(seed["operations-center.json"])

    if "trusted_client_certificates" not in oc_json:
        oc_json["trusted_client_certificates"] = []

    oc_json["trusted_client_certificates"].append(client_cert_pub.decode("UTF-8"))

    seed["operations-center.json"] = json.dumps(oc_json)

    # Configure the images provider, if needed
    IMAGES_SERVER = os.getenv("IMAGES_SERVER")
    if IMAGES_SERVER is not None:
        if "provider.json" not in seed:
            seed["provider.json"] = '{"name":"images","config":{"server_url":"' + IMAGES_SERVER + '/os"}}'

    # Inject seed data.
    with open(test_image, "rb+") as f:
        f.seek(4196352*512)

        with tarfile.open(mode="w", fileobj=f) as tar:
            for filename, contents in seed.items():
                raw = contents.encode("utf-8")
                buf = io.BytesIO(raw)
                ti = tarfile.TarInfo(name=filename)
                ti.size = len(raw)

                tar.addfile(ti, buf)

    # Return the path of the customized install image, the OS name, the OS version, and a path to the temporary TLS client certificate.
    parts = basename.split("_")
    return test_image, parts[0], parts[1].replace(ext, ""), client_cert_name

def _manual_download_application(directory, name, version):
    IMAGES_SERVER = os.getenv("IMAGES_SERVER", "https://images.linuxcontainers.org")

    os.mkdir(directory+"/update")

    urllib.request.urlretrieve(IMAGES_SERVER + "/os/"+version+"/update.json", directory+"/update/update.json")
    urllib.request.urlretrieve(IMAGES_SERVER + "/os/"+version+"/update.sjson", directory+"/update/update.sjson")

    os.mkdir(directory+"/update/x86_64")

    with open(directory+"/update/update.json") as f:
        j = json.load(f)

        for updateFile in j["files"]:
            if updateFile["architecture"] != "x86_64":
                continue

            if updateFile["type"] != "application" or updateFile["component"] != name:
                continue

            urllib.request.urlretrieve(IMAGES_SERVER + "/os/"+version+"/"+updateFile["filename"], directory+"/update/"+updateFile["filename"])

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
