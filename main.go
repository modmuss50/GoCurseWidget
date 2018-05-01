package main

import (
	"github.com/modmuss50/goutils"
	"encoding/json"
	"net/http"
	"io"
	"html/template"
	"github.com/dustin/go-humanize"
	"regexp"
	"github.com/patrickmn/go-cache"
	"time"
	"fmt"
	"errors"
)

var (
	Cache *cache.Cache
)

func main() {
	//Creates a 30 min cache, cleans up every 1 min
	Cache = cache.New(30*time.Minute, 1*time.Minute)

	http.HandleFunc("/widget/", widgetResponse)
	http.ListenAndServe(":8000", nil)
}

func widgetResponse(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("www/widget.html")
	if err != nil {
		io.WriteString(w, "An error occurred")
		return
	}
	regex, err := regexp.Compile("[^/]+$")
	if err != nil {
		io.WriteString(w, "An error occurred")
		return
	}
	projectID := string(regex.Find([]byte(r.URL.String())))
	if projectID == "" {
		io.WriteString(w, "No or invalid project id provided")
		return
	}

	projectData, found := Cache.Get(projectID)
	if !found {
		fmt.Println("Loading " + projectID)
		project, err := getProjectData(projectID)
		if err != nil {
			io.WriteString(w, "An error occurred when loading curse data")
			return
		}
		Cache.Set(projectID, project, cache.DefaultExpiration)
		projectData = project
	} else {
		fmt.Println("Using cached projectID")
	}

	tmpl.Execute(w, projectData)
}

func getProjectData(projectID string) (*ProjectData, error) {
	addonBytes, err := goutils.Download("https://cursemeta.dries007.net/api/v2/direct/GetAddOn/" + projectID)
	if err != nil {
		return nil, err
	}
	var addonData *ProjectData
	if err := json.Unmarshal(addonBytes, &addonData); err != nil {
		return nil, err
	}
	if addonData == nil {
		return nil, errors.New("failed to load curse addondata")
	}
	//Populate the extra fields I added to make things easier for the template
	for _, attachment := range addonData.Attachments {
		if attachment.IsDefault {
			addonData.Thumbnail = attachment.URL
		}
	}
	addonData.DownloadCountPretty = humanize.Comma(int64(addonData.DownloadCount))


	return addonData, nil
}

//Made with https://mholt.github.io/json-to-go/
type ProjectData struct {
	Thumbnail           string //Not in json, moved here to make things easier
	DownloadCountPretty string //This is a nice looking download count


	Attachments []struct {
		Description  interface{} `json:"Description"`
		IsDefault    bool        `json:"IsDefault"`
		ThumbnailURL string      `json:"ThumbnailUrl"`
		Title        string      `json:"Title"`
		URL          string      `json:"Url"`
	} `json:"Attachments"`
	Authors []struct {
		Name string `json:"Name"`
		URL  string `json:"Url"`
	} `json:"Authors"`
	AvatarURL interface{} `json:"AvatarUrl"`
	Categories []struct {
		ID   int    `json:"Id"`
		Name string `json:"Name"`
		URL  string `json:"URL"`
	} `json:"Categories"`
	CategorySection struct {
		ExtraIncludePattern     interface{} `json:"ExtraIncludePattern"`
		GameID                  int         `json:"GameID"`
		ID                      int         `json:"ID"`
		InitialInclusionPattern string      `json:"InitialInclusionPattern"`
		Name                    string      `json:"Name"`
		PackageType             string      `json:"PackageType"`
		Path                    string      `json:"Path"`
	} `json:"CategorySection"`
	CommentCount       int         `json:"CommentCount"`
	DefaultFileID      int         `json:"DefaultFileId"`
	DonationURL        interface{} `json:"DonationUrl"`
	DownloadCount      float64     `json:"DownloadCount"`
	ExternalURL        interface{} `json:"ExternalUrl"`
	GameID             int         `json:"GameId"`
	GamePopularityRank int         `json:"GamePopularityRank"`
	GameVersionLatestFiles []struct {
		FileType        string `json:"FileType"`
		GameVesion      string `json:"GameVesion"`
		ProjectFileID   int    `json:"ProjectFileID"`
		ProjectFileName string `json:"ProjectFileName"`
	} `json:"GameVersionLatestFiles"`
	IconID       int `json:"IconId"`
	ID           int `json:"Id"`
	InstallCount int `json:"InstallCount"`
	IsFeatured   int `json:"IsFeatured"`
	LatestFiles []struct {
		AlternateFileID int `json:"AlternateFileId"`
		Dependencies []struct {
			AddOnID int    `json:"AddOnId"`
			Type    string `json:"Type"`
		} `json:"Dependencies"`
		DownloadURL    string   `json:"DownloadURL"`
		FileDate       string   `json:"FileDate"`
		FileName       string   `json:"FileName"`
		FileNameOnDisk string   `json:"FileNameOnDisk"`
		FileStatus     string   `json:"FileStatus"`
		GameVersion    []string `json:"GameVersion"`
		ID             int      `json:"Id"`
		IsAlternate    bool     `json:"IsAlternate"`
		IsAvailable    bool     `json:"IsAvailable"`
		Modules []struct {
			Fingerprint int    `json:"Fingerprint"`
			Foldername  string `json:"Foldername"`
		} `json:"Modules"`
		PackageFingerprint int    `json:"PackageFingerprint"`
		ReleaseType        string `json:"ReleaseType"`
	} `json:"LatestFiles"`
	Likes                    int     `json:"Likes"`
	Name                     string  `json:"Name"`
	PackageType              string  `json:"PackageType"`
	PopularityScore          float64 `json:"PopularityScore"`
	PrimaryAuthorName        string  `json:"PrimaryAuthorName"`
	PrimaryCategoryAvatarURL string  `json:"PrimaryCategoryAvatarUrl"`
	PrimaryCategoryID        int     `json:"PrimaryCategoryId"`
	PrimaryCategoryName      string  `json:"PrimaryCategoryName"`
	Rating                   int     `json:"Rating"`
	Stage                    string  `json:"Stage"`
	Status                   string  `json:"Status"`
	Summary                  string  `json:"Summary"`
	WebSiteURL               string  `json:"WebSiteURL"`
}
