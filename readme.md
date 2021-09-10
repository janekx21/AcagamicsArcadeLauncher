# Acagamics Arcade Launcher

A simple configurable games launcher written in go.

# How to configure

## `./games` folder

The folder contains all games and meta information.

```
root/
├─ launcher.exe
└─ games/
   ├─ chickenrun/
   │  ├─ meta.json
   │  ├─ thumbnail.png
   │  └─ ChickenRun.exe
   │
   └─ another game/
      ├─ meta.json
      ├─ other_thumbnail.png
      └─ other_game.exe
```

In this case we have two games installed.

## `meta.json` files

This file describes a game that can be shown.

```json
{
  "name": "Chicken Run",
  "author": "Janek",
  "release_date": "2021-09-07T23:19:29.6261973+02:00",
  "thumbnail_path": "./thumbnail.png",
  "executable_path": "./ChickenRun.exe"
}
```

# How to get dependencies

```
go get
```

Pulls all dependencies.

# How to build

```
go build
```

Builds an executable.