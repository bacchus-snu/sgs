# sgs

## Development environment

There are two ways to configure the development environment:

### Nix

```console
$ nix develop
````

### Not Nix

Prerequisites:

- Go >= 1.22
- NodeJS

```console
# install build dependencies
$ make build-deps
```

## Development

```
# install npm dependencies
$ npm i

# hot reloader
$ air

# running tests:
$ docker-compose up -d
$ make check
```
