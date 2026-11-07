# Shared API

Each IncusOS service shares a common API that can be used to get its state and configuration, update its configuration, perform service-specific actions, and forcefully reset the service if needed.

## Listing the services

```
incus admin os service list
```

## Getting the service state and configuration

```
incus admin os service show <name>
```

## Editing the service configuration

```
incus admin os service edit <name>
```

## Performing service actions

Some services support additional actions, which are listed on the page of the service. An action is triggered by providing its name and an optional map of parameters

```
incus admin os service action <name> -d '{"action":"<action>","config":{"<key>":"<value>"}}'
```

## Resetting the application

If needed, a service can be forcefully reset by running

```
incus admin os service reset <name>
```
