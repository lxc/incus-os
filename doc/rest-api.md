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
  
  * `/1.0/debug`
  
    `GET`: Returns a list of debug endpoints
  
  * `/1.0/debug/log`
  
    `GET`: Returns systemd journal entries, optionally filtering by unit, boot number, and
    number of return entries
  
  * `/1.0/services`
  
    `GET`: Returns a list of service endpoints
  
  * `/1.0/services/{name}`
  
    `GET`: Returns service-specific status and/or configuration information
    
    `POST`: Update a service's configuration
  
  * `/1.0/system`
  
    `PUT`: Perform a system-wide action
      - `shutdown`, `poweroff`, `reboot`: Self-descriptive
      - `reset_encryption_bindings`: Force-reset TPM encryption bindings; intended only for
      use if it was required to enter a recovery passphrase to boot the system
  
  * `/1.0/system/network`
  
    `GET`: Return the current network state
    
    `PATCH`: Update the current network configuration
    
    `PUT`: Replace the current network configuration
  
  * `/1.0/system/resources`
  
    `GET`: Returns a detailed low-level dump of the system's resources
  
  * `/1.0/system/security`
  
    `GET`: Returns information about the system's security state, such as Secure Boot and TPM
    status, encryption recovery keys, etc
    
    `PUT`: Add an encryption recovery passphrase
    
    `DELETE`: Remove an encryption recovery passphrase
  
  * `/1.0/system/storage`
  
    `DELETE`: Destroy a local storage pool
  
    `GET`: Returns information about drives present in the system and status of any local storage
    pools
    
    `PUT`: Create or update a local storage pool
  
  * `/1.0/system/storage/wipe`
  
    `POST`: Forcibly wipe all data from the specified drive
  
  * `/1.0/system/update`

    `GET`: Returns the current system update state
    
    `POST`: Trigger an immediate system update check
    
    `PUT`: Apply a new system update configuration
