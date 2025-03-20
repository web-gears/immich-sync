
### Minimal space usage option

You can minimize use of your disk space by extracting only `.json` files required for synchronization.
You'll need `7zip` installed for that. After ensuring it's in your system PATH, just execute this command inside your download folder, replacing `takeout-XXXX-XX-XXT000000Z` with your archive file pattern:


```shell
7z x takeout-XXXX-XX-XXT000000Z-*.zip -o"out" *.json -r
```