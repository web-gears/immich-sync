package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type Asset struct {
	Title *string `json:"title"`
}

type ImmichAlbumAsset struct {
	Assets ImmichAssetAssets `json:"assets"`
}

type ImmichAssetAssets struct {
	Total int           `json:"total"`
	Count int           `json:"count"`
	Items []ImmichAsset `json:"items"`
}

type ImmichAsset struct {
	Id               string `json:"id"`
	Type             string `json:"type"`
	OriginalFileName string `json:"originalFileName"`
	OriginalMimeType string `json:"originalMimeType"`
}

type Album struct {
	AlbumName string `json:"albumName"`
	Id        string `json:"id"`
	OwnerId   string `json:"ownerId"`
	Assets    []struct {
		Id               string `json:"id"`
		OriginalFileName string `json:"originalFileName"`
		OriginalMimeType string `json:"originalMimeType"`
	} `json:"assets"`
}

type Takeout struct {
	Albums []struct {
		Name   string `json:"name"`
		Assets []struct {
			Filename string `json:"filename"`
		}
	} `json:"albums"`
}

var takeout map[string][]string

var apiURL string = ""
var apiKey string = ""
var takeoutPath string = ""

var supressConfirmation = false

func getData(path string, method string, payload string) (body []byte, err error) {
	url := apiURL + path
	client := &http.Client{}
	req, err := http.NewRequest(method, url, strings.NewReader(payload))
	if apiKey == "" {
		fmt.Println("No API key set")
		os.Exit(1)
	}

	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("x-api-key", apiKey)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err = io.ReadAll(res.Body)
	if err != nil {

		return nil, err
	}
	return body, nil
}

func YesNoPrompt(label string, def bool, required bool) bool {
	if !required && supressConfirmation {
		return true
	}
	choices := "Y/n"
	if !def {
		choices = "y/N"
	}

	r := bufio.NewReader(os.Stdin)
	var s string

	for {
		fmt.Fprintf(os.Stderr, "%s (%s) ", label, choices)
		s, _ = r.ReadString('\n')
		s = strings.TrimSpace(s)
		if s == "" {
			return def
		}
		s = strings.ToLower(s)
		if s == "y" || s == "yes" {
			return true
		}
		if s == "n" || s == "no" {
			return false
		}
	}
}

func getAlbums() (albums []Album, err error) {
	fmt.Println("Getting albums...")
	body, err := getData("albums", "GET", "")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	json.Unmarshal(body, &albums)
	fmt.Printf("%s\n", fmt.Sprintf("Total albums: %d", len(albums)))
	return albums, nil
}

func readFiles() {
	basePath := takeoutPath
	entries, err := os.ReadDir(basePath)
	if err != nil {
		log.Fatal(err)
	}
	if takeout == nil {
		takeout = make(map[string][]string)
	}
	for _, entry := range entries {
		entryPath := basePath + "/" + entry.Name()
		metadata, err := os.Open(entryPath + "/metadata.json")
		if err == nil {
			bytefile, _ := io.ReadAll(metadata)
			defer metadata.Close()
			var asset Asset
			json.Unmarshal(bytefile, &asset)
			if asset.Title != nil && *asset.Title != "" {
				readAlbum(entryPath, *asset.Title)
			}

		}
	}

}

func readAlbum(albumPath string, albumTitle string) {

	fmt.Println("Found album: " + albumTitle)
	entries, err := os.ReadDir(albumPath)
	if err != nil {
		log.Fatal(err)
	}
	if _, ok := takeout[albumTitle]; !ok {
		takeout[albumTitle] = []string{}
	}
	for _, entry := range entries {
		if entry.Name() == "metadata.json" || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		file, err := os.Open(albumPath + "/" + entry.Name())
		if err == nil {
			bytefile, _ := io.ReadAll(file)
			defer file.Close()
			var asset Asset
			json.Unmarshal(bytefile, &asset)
			if asset.Title != nil {
				takeout[albumTitle] = append(takeout[albumTitle], *asset.Title)
			}

		}
	}
}

func findAssetByFilename(filename string) (asset []ImmichAsset) {
	body, err := getData("search/metadata", "POST", "{\"originalFileName\": \""+filename+"\"}")
	if err != nil {
		fmt.Println(err)
	}
	var response ImmichAlbumAsset
	json.Unmarshal(body, &response)
	if response.Assets.Total == 0 {
		fmt.Println("Asset not found: " + filename)
		return
	}
	return response.Assets.Items
}

func syncAlbum(album Album, files []string, override bool) {
	fmt.Println("Syncing album: " + album.AlbumName)
	if !override {
		ok := YesNoPrompt("Are you sure you want to sync this album?", true, false)
		if !ok {
			return
		}
	}
	body, err := getData("albums/"+album.Id, "GET", "")
	if err != nil {
		fmt.Println(err)
		return
	}
	newAssets := []string{}
	var immichAlbum Album
	json.Unmarshal(body, &immichAlbum)
	for _, file := range files {
		found := false
		for _, asset := range immichAlbum.Assets {
			if asset.OriginalFileName == file {
				found = true
				break
			}
		}
		if !found {
			fmt.Println("Adding file: " + file)
			assets := findAssetByFilename(file)
			var ids []string
			for _, asset := range assets {
				ids = append(ids, asset.Id)

			}

			newAssets = append(newAssets, ids...)
			continue
		}
	}
	if len(newAssets) > 0 {
		ids, _ := json.Marshal(newAssets)
		body, err := getData("albums/"+album.Id+"/assets", "PUT", "{\"ids\": "+string(ids)+"}")

		if err != nil {
			fmt.Println(err)
			return
		}
		var response []struct {
			Id      string `json:"id"`
			Success bool   `json:"success"`
		}
		json.Unmarshal(body, &response)
		for _, r := range response {
			if !r.Success {
				fmt.Println("Error syncing asset: " + r.Id)
			}

		}
	}
}

func createAlbum(albumName string, files []string) {
	fmt.Println("Creating album: " + albumName)
	ok := YesNoPrompt("Are you sure you want to create this album?", true, false)
	if !ok {
		return
	}
	body, err := getData("albums", "POST",
		"{\"albumName\": \""+albumName+"\"}")
	if err != nil {
		fmt.Println(err)
		return
	}
	var immichAlbum Album
	json.Unmarshal(body, &immichAlbum)
	fmt.Printf("New album created: %s\n", immichAlbum.Id)
	syncAlbum(immichAlbum, files, true)
}

func readConfig() {
	file, err := os.Open("config.json")
	if err == nil {
		bytefile, _ := io.ReadAll(file)
		defer file.Close()
		var config struct {
			ApiKey      string `json:"apiKey"`
			ApiURL      string `json:"apiURL"`
			TakeoutPath string `json:"takeoutPath"`
		}
		json.Unmarshal(bytefile, &config)
		apiKey = config.ApiKey
		apiURL = config.ApiURL
		takeoutPath = config.TakeoutPath
	} else {
		fmt.Println("No config file found")
		os.Exit(1)
	}
}

func main() {
	readConfig()

	if len(os.Args) > 1 && os.Args[1] == "-y" {
		supressConfirmation = true
	}

	body, err := getData("users/me", "GET", "")
	if err != nil {
		fmt.Println(err)
	}
	var response struct {
		Id   string `json:"id"`
		Name string `json:"name"`
	}
	json.Unmarshal(body, &response)

	ok := YesNoPrompt("Are you "+response.Name+"?", false, true)
	if !ok {
		fmt.Println("Bye-bye...")
		return
	}

	readFiles()
	albums, err := getAlbums()
	for albumName, files := range takeout {
		foundAlbum := false
		for _, album := range albums {
			if album.AlbumName == albumName {
				syncAlbum(album, files, false)
				foundAlbum = true
				break
			}
		}
		if !foundAlbum {
			createAlbum(albumName, files)
		}
	}
	if err != nil {
		fmt.Println(err)
	} else {
	}
	fmt.Println("Done!")
}
