# Incus OS API

**WARNING** The Incus OS API is not yet considered stable and may change in ways that
are not backwards compatible.

## REST endpoints

  * `/1.0`
  
    `GET`: Returns the system's hostname, OS name, and OS version
 
  * `/1.0/applications`
  
    `GET`: Returns a list of installed application endpoints
  
  * `/1.0/applications/{name}`
  
    `GET`: Returns application-specific status and/or configuration information

  * `/1.0/applications/{name}/:backup`

    `POST`: Returns a tar archive backup for the application. If passed `complete=true`, a
    full backup will be generated which may be quite large depending on what artifacts or
    updates may be locally cached by the application.

  * `/1.0/applications/{name}/:factory-reset`

    `POST`: Performs a factory reset of the application

  * `/1.0/applications/{name}/:restore`

    `POST`: Restore a tar archive backup for the application

    Remember to properly set the 'Content-Type: application/x-tar' HTTP header.

  * `/1.0/debug`
  
    `GET`: Returns a list of debug endpoints
  
  * `/1.0/debug/log`
  
    `GET`: Returns systemd journal entries, optionally filtering by unit, boot number, and
    number of return entries

  * `/1.0/debug/tui/:write-message`
  
    `POST`: Send a message that should be logged by the system

  * `/1.0/services`

    `GET`: Returns a list of service endpoints

  * `/1.0/services/{name}`

    `GET`: Returns service-specific status and/or configuration information

    `PUT`: Update a service's configuration

  * `/1.0/system`

    `GET`: Returns a list of system endpoints

  * `/1.0/system/:backup`

    `POST`: Return a tar archive backup of the system state and configuration

  * `/1.0/system/:factory-reset`

    `POST`: Perform a complete factory reset of the system and immediately reboot. An
    optional number of seed configurations may also be provided, which will be used
    when the system starts up into its fresh state.

  * `/1.0/system/:poweroff`

    `POST`: Power off the system

  * `/1.0/system/:reboot`

    `POST:` Reboot the system

  * `/1.0/system/:restore`

    `POST`: Use provided tar archive to perform a restore of the system state and
    configuration. Upon completion the system will immediately reboot.

    Optionally, a `skip` parameter may be provided consisting of a comma-separated
    list of items to ignore when restoring the backup. Supported values include:

      - "encryption-recovery-keys": Do not overwrite any existing encryption recovery
        keys
      - "local-data-encryption-key": Do not overwrite the existing encryption key for
        the "local" data pool
      - "network-macs": Do not copy MAC addresses from network interface or bond
        definitions in the backup

    Remember to properly set the 'Content-Type: application/x-tar' HTTP header.

  * `/1.0/system/logging`

    `GET`: Returns the current system logging state

    `PUT`: Apply a new system logging configuration

  * `/1.0/system/network`

    `GET`: Return the current network state

    `PUT`: Apply a new network configuration

  * `/1.0/system/provider`

    `GET`: Returns the current system provider state

    `PUT`: Apply a new system provider configuration

  * `/1.0/system/resources`

    `GET`: Returns a detailed low-level dump of the system's resources

  * `/1.0/system/security`

    `GET`: Returns information about the system's security state, such as Secure Boot and TPM
    status, encryption recovery keys, etc

    `PUT`: Update list of encryption recovery keys

  * `/1.0/system/security/:tpm-rebind`

    `POST`: Force-reset TPM encryption bindings; intended only for use if it was required to enter
    a recovery passphrase to boot the system

  * `/1.0/system/storage`

    `GET`: Returns information about drives present in the system and status of any local storage
    pools

    `PUT`: Create or update a local storage pool

  * `/1.0/system/storage/:delete-pool`

    `POST`: Destroy a local storage pool

  * `/1.0/system/storage/:import-encryption-key`

    `POST`: Set the encryption key when importing an existing storage pool

  * `/1.0/system/storage/:wipe-drive`

    `POST`: Forcibly wipe all data from the specified drive

  * `/1.0/system/update`

    `GET`: Returns the current system update state

    `PUT`: Apply a new system update configuration

  * `/1.0/system/update/:check`

    `POST`: Trigger an immediate system update check
