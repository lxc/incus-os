# Licenses

IncusOS is built from a large number of open source components. The copyright and license
information for everything present on a running system can be retrieved through the API.

The list of available entries can be obtained by running

```
incus admin os license list
```

Each entry has a type and a name.

| Type      | Content                                                                    |
| --------- | -------------------------------------------------------------------------- |
| `debian`  | Copyright file of a Debian package.                                        |
| `local`   | License of software that is built or included outside of a Debian package. |
| `license` | Full text of a common license.                                             |

The copyright file of a Debian package will often only refer to a common license by name.
The full text of that license is then available through the matching `license` entry.

To get the content of an entry, pass its type and name

```
incus admin os license show debian systemd
```

The list covers the base system as well as every installed application, so it will change as
applications are added or removed.
