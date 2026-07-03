# Installing IncusOS on an OpenStack Cloud

It is possible to install **IncusOS** with full security on OpenStack-based
cloud platforms. This guide was tested on the **Infomaniak Public Cloud**,
but the steps are applicable to most OpenStack-compatible providers worldwide.

```{warning}
Most operations on OpenStack instances - such as **adding/modifying
volumes, RAM, or CPUs** - will trigger the **TPM (Trusted Platform Module)**.

- **Download the TPM unlock password** as soon as possible.
- **Keep it secure and accessible**, as after instance changes you will
  need to enter it during boot via the OpenStack Console.
```

## 1. Order and Prepare a Cloud Project

1. **Order a public cloud** and create a project within it.
1. **Enable OpenStack access** for your project. This provides access to
   the **Horizon dashboard**, where you can:
   - Create instances
   - Add volumes
   - Configure networks
   - Manage other cloud resources

## 2. Obtain a Suitable IncusOS Image

1. Follow the instructions to **[download an IncusOS image](../download.md)**.
1. Use the **Web Customizer** to generate a **USB operation mode image**
   (required for direct execution).
1. In the customization form:
   - **Add the required TLS client certificate** (mandatory).
   - Fill in the remaining fields as needed (optional).
1. Download the generated image.

## 3. Process the Image

### Convert the Image to `QCOW2` Format

OpenStack requires the image to be in **`QCOW2` format**.
Convert the downloaded image locally using the following command:

```bash
qemu-img convert -f raw -O qcow2 IncusOS_202605181246.img incusos-openstack.qcow2
```

### Upload the Image to OpenStack

Use the **OpenStack command line tool** and your credentials to upload the converted image:

```bash
openstack image create "IncusOS-Hypervisor" \
  --file incusos-openstack.qcow2 \
  --disk-format qcow2 \
  --container-format bare \
  --property hw_firmware_type=uefi \
  --property hw_tpm_version=2.0 \
  --property hw_tpm_model=tpm-tis
```

> **Note:** Explicitly configuring **UEFI** and **TPM 2.0**
> is required for IncusOS compatibility.

## 4. Create an OpenStack Instance

1. **Select an instance type** with a minimum of **50GB storage** (IncusOS will run on most instance types meeting this requirement).
1. **Choose the uploaded image** (`IncusOS-Hypervisor`) as the boot source.
1. **Attach a network** to the instance.
1. **Create the instance**.

### Configure Security Group

Modify the default OpenStack **security group** to allow inbound traffic on
**port 8443**, where IncusOS listens for connections.

## 5. Access IncusOS

1. The system will **boot automatically** into IncusOS.
1. Access the **OpenStack Console** in your browser to monitor the boot process.
1. Once the system is ready, follow the **[access instructions](../access.md)** to log in.
