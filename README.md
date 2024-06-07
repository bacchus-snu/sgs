# sgs

## Development environment

There are two ways to configure the development environment:

### Nix

```console
$ nix develop
````

## Development

```
$ # install npm dependencies
$ npm i

$ # hot reloader
$ make hotreload

$ # run tests
$ docker-compose up -d
$ make check

$ # serve docs locally
$ make serve-docs
```
