# Shared API

Each IncusOS service shares a common API that can be used to get its state and configuration, update its configuration, and forcefully reset the service if needed.

## Getting the service state and configuration

```
incus admin os service show <name>
```

## Editing the service configuration

```
incus admin os service edit <name>
```

## Resetting the application

If needed, a service can be forcefully reset by running

```
incus admin os service reset <name>
```
