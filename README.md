# Google Photo to Immich sync

## Description

This tool is not for importing entire Google Photos takeout to immich (for that you can use [immich-go](https://github.com/simulot/immich-go)).
This tool is to sync albums from Google Photos to existing Immich library.

It also written in Go, just for fun.

### Use case
Let's say you have your photos backup configured to both Immich and Google Photos; or you just imported your old (i.e. Nextcloud) library to Immich, but you created albums only in Google Photos.

In both places you have similar set of assets, and want to organize your hi-rez originals the same way as your (most likely compressed) library in Google Photos.

You don't want to re-import the whole library every time you download Google Takeout, you just want to sync your albums every now and then.

### Support Immich sync

Become [GitHub Sponsor](https://github.com/sponsors/web-gears)

Support with [PayPal](https://www.paypal.com/donate/?business=8879BAAHSFANQ&no_recurring=0&currency_code=USD)

## How to use

### Prerequisites

Currently, script runs from the project's root and doesn't support installation.

- First, you need to setup Go: https://go.dev/doc/install
- Then, clone this repo and go into the root folder with `main.go`
- Run `go install` from the project root and add `IMMICH_API_KEY` and `IMMICH_API_URL` environment variables.
- Add `IMMICH_TAKEOUT_PATH` with full path to your Google Photos takeout folder
or go to that folder to run `immich-sync`.

#### Without installation
- If you don't want to install and add environment variables, then create `config.json` in the project root with the following content:
```json
{
    "apiKey": "YourImmichApiKey",
    "apiURL": "http://your_immich_host:port/api/",
    "takeoutPath": "../Takeout/Google Photos"
}
```

### Usage
Finally, run:
```shell
immich-sync
```
or (if you want to run it without installation)
```shell
go run main.go
```
First, script will call your Immich instance API and verify the name of the API key owner.

After that, it will scan your Google Photo takeout folder and find json files for albums with photos eligible to synchronization.

Then, it will go over all albums asking if you want to sync photos to existing Immich album if it finds one, or create a new one. Every album synchonization require confirmation in command prompt. You can skip album confirmations by running the app with the ` -y` argument.
Albums synchromization will not remove existing photos from Immich albums - it will just append missing ones. 

### Files dates synchronization

There's an option to sync assets dates for one day with the data from takeout json files. Sometimes google includes files with broken metadata, leaving proper dates inthe json files.
For this, you need to skip album sync, and the next option would be to sync files. After that, you will be prompted with the dialog to enter the date. If date mismatch found between assets at this day and info from takeout, you will have an option to update assets with the takeout date/time.

### Limitations

Assets for synchronization searched by filename and creation time (+/- one day). That means that if you have different assets with identical filenames taken at the same day - this method could produce false positive search and potentially add extra or wrong assets to the albums.

### Minimal space usage option

Because pp relies only on `.json` files from Google Photos takeout, you can minimize use of your disk space by extracting only files required for synchronization.
You'll need `7zip` installed for that. After ensuring it's in your system PATH, just execute this command inside your download folder, replacing `takeout-XXXX-XX-XXT000000Z` with your archive file pattern:

```shell
7z x takeout-XXXX-XX-XXT000000Z-*.zip -o"out" *.json -r
```
This will extract Google Takeout archive including only `.json` files.

## To Do
- Add CLI arguments support for configuration parameters