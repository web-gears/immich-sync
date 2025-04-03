package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Asset struct {
	Title        *string `json:"title"`
	CreationTime struct {
		Timestamp string `json:"timestamp"`
	} `json:"creationTime"`
	PhotoTakenTime struct {
		Timestamp string `json:"timestamp"`
	} `json:"photoTakenTime"`
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

type TakeoutFile struct {
	Filename       string
	CreationTime   string
	PhotoTakenTime string
}

var takeout map[string][]TakeoutFile

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
	fmt.Println("Getting Immich albums...")
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
		takeout = make(map[string][]TakeoutFile)
	}
	fmt.Println("Reading takeout albums...")
	albumsTotal := 0
	for _, entry := range entries {
		entryPath := basePath + "/" + entry.Name()
		metadata, err := os.Open(entryPath + "/metadata.json")
		if err == nil {
			bytefile, _ := io.ReadAll(metadata)
			defer metadata.Close()
			var asset Asset
			json.Unmarshal(bytefile, &asset)
			if asset.Title != nil && *asset.Title != "" {
				albumsTotal++
				readAlbum(entryPath, *asset.Title)
			}

		}
	}
	if albumsTotal == 0 {
		fmt.Println("No albums found")
	} else {
		fmt.Printf("Found %d albums\n", albumsTotal)
	}

}

func readAlbum(albumPath string, albumTitle string) {

	fmt.Println("Found album: " + albumTitle)
	entries, err := os.ReadDir(albumPath)
	if err != nil {
		log.Fatal(err)
	}
	if _, ok := takeout[albumTitle]; !ok {
		takeout[albumTitle] = []TakeoutFile{}
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
				takeout[albumTitle] = append(takeout[albumTitle], TakeoutFile{Filename: *asset.Title, CreationTime: asset.CreationTime.Timestamp, PhotoTakenTime: asset.PhotoTakenTime.Timestamp})
			}

		}
	}
}

func findAssetByFilename(filename string, createdAfter string, createdBefore string) (asset []ImmichAsset) {
	body, err := getData("search/metadata", "POST", "{\"originalFileName\": \""+filename+"\", \"takenAfter\": \""+createdAfter+"\", \"takenBefore\": \""+createdBefore+"\"}")
	if err != nil {
		fmt.Println(err)
	}
	var response ImmichAlbumAsset
	json.Unmarshal(body, &response)
	if response.Assets.Total == 0 {
		// fmt.Println("Asset not found: " + filename)
		return
	}
	return response.Assets.Items
}

func syncAlbum(album Album, files []TakeoutFile, override bool) {
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
	notFound := 0
	fmt.Print("Searching files ")
	for _, file := range files {
		found := false
		for _, asset := range immichAlbum.Assets {
			if asset.OriginalFileName == file.Filename {
				found = true
				break
			}
		}
		if !found {
			fmt.Print(".")
			createdAfterTimestamp, err1 := strconv.ParseInt(file.PhotoTakenTime, 10, 64)
			createdBeforeTimestamp, err2 := strconv.ParseInt(file.CreationTime, 10, 64)
			if err1 != nil || err2 != nil {
				fmt.Println("Error parsing timestamp:", err)
				return
			}
			tm1 := time.Unix(createdAfterTimestamp, 0)
			tm2 := time.Unix(createdBeforeTimestamp, 0)
			createdAfter := tm1.Add(time.Hour * 24 * -1).Format("2006-01-02")
			createdBefore := tm2.Add(time.Hour * 24 * 1).Format("2006-01-02")

			assets := findAssetByFilename(file.Filename, createdAfter, createdBefore)
			if len(assets) == 0 {
				notFound++
			}
			var ids []string
			for _, asset := range assets {
				ids = append(ids, asset.Id)

			}

			newAssets = append(newAssets, ids...)
			continue
		}
	}
	fmt.Println(" Done")
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
		i := 0
		for _, r := range response {
			if !r.Success {
				fmt.Println("Error syncing asset: " + r.Id)
			} else {
				i++
			}
		}
		fmt.Println("Synced " + strconv.Itoa(i) + " assets to " + album.AlbumName)
	} else {
		fmt.Println("No new assets added to " + album.AlbumName)
	}
	if notFound > 0 {
		fmt.Println("Warning: " + strconv.Itoa(notFound) + " assets not found")
	}
}

func createAlbum(albumName string, files []TakeoutFile) {
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
		fmt.Println("No config file found, trying ENV vars...")
		apiKey = os.Getenv("IMMICH_API_KEY")
		apiURL = os.Getenv("IMMICH_API_URL")
		takeoutPath = os.Getenv("IMMICH_TAKEOUT_PATH")
		if apiKey == "" {
			fmt.Println("No IMMICH_API_KEY set")
			os.Exit(1)
		}
		if apiURL == "" {
			fmt.Println("No IMMICH_API_URL set")
			os.Exit(1)
		}
		if takeoutPath == "" {
			fmt.Println("No IMMICH_TAKEOUT_PATH set, using current directory")
			takeoutPath = "."
		}

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
	if len(takeout) == 0 {
		fmt.Println("No files found")
		return
	}
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
