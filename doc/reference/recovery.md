# System recovery

IncusOS is designed to be fairly resilient to failures, but there may be times when
your system misbehaves. Here are some suggestions that might be useful if you ever
need to recover an IncusOS system.

## Try booting into the previous image

IncusOS uses an A/B update mechanism to reboot onto the newer version while keeping
the previous version available should a revert be needed. You can reboot your server
and select the prior version at the boot menu. If that works, it means that something
went wrong with the latest update -- please report a bug!

## Encryption recovery key(s)

IncusOS binds encryption of the install drive to the system's TPM state and stores any
additional local pool encryption keys on that encrypted drive. You can retrieve an
encryption recovery passphrase for the install drive as well as any local pool encryption
keys via the API. (You did do that and saved those somewhere safe _before_ we ended up here,
right?)

If something unexpectedly changes the TPM state of your system, you can still boot but
will need to manually provide an encryption recovery passphrase. After IncusOS starts
up, you can use the API to force-reset the TPM encryption bindings which should allow
automatic decryption of the install drive at boot time.

Alternatively, with the recovery key(s), you can remove the affected drive(s) to a different
machine and unlock them to access/migrate any data they contain.

## Drive failure

If your install drive fails, sorry but there's not much that can be done other than a
new install. :(

If a drive in a local storage pool fails, and the pool has sufficient redundancy, you can
remove the failed drive and replace it with a new one via the API. The underling pool driver
will begin data recovery process(es), which you can monitor via querying the status of the
storage endpoint.

## Recovery mode

A special "recovery mode" can be triggered early in the IncusOS boot sequence if a data partition
labeled `RESCUE_DATA` and formatted as FAT or ISO is present. IncusOS will automatically
attempt to find and run a hot-fix script named `hotfix.sh.sig` at the root of that partition,
followed by any OS or application updates contained in an `update/` directory also at the root
of the recovery partition.

Both the hot-fix script and update metadata JSON file must be properly signed by the same
certificate used to distribute normal IncusOS updates. This prevents an attacker from simply
being able to connect a random USB stick and then running arbitrary commands with full
system access.

The recovery mode is intended as an option of last resort.
