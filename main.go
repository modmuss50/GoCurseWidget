package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/blang/semver"
	"github.com/dustin/go-humanize"
	"github.com/generaltso/vibrant"
	"github.com/modmuss50/CAV2"
	"github.com/modmuss50/goutils"
	"github.com/patrickmn/go-cache"
	"github.com/paulbellamy/ratecounter"
	"gopkg.in/go-playground/colors.v1"
	"html/template"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"
)

var (
	Cache          *cache.Cache
	HistoryCache   *cache.Cache
	RateCounter    *ratecounter.RateCounter
	LastResponse   string
	WidgetTemplate string
	DirectDownload bool
)

const Port = "8888"

func main() {
	//Creates a 30 min cache, cleans up every 1 min
	Cache = cache.New(30*time.Minute, 1*time.Minute)
	//Creates a 24 hour cache, cleans every 30 mins
	HistoryCache = cache.New(24*time.Hour, 30*time.Minute)
	//Creates a rate counted used to show counts per hour
	RateCounter = ratecounter.NewRateCounter(1 * time.Hour)
	//Stores the last response time
	LastResponse = "0"

	//Loads cav
	cav2.SetupDefaultConfig()

	//Sets up the logger
	//openLogFile("gocurse.log")
	//log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	fmt.Println("Starting at http://localhost:" + Port)

	http.HandleFunc("/", index)
	http.HandleFunc("/widget/", widgetResponse)
	http.ListenAndServe(":"+Port, http.DefaultServeMux)
}

func index(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("www/index.html")
	if err != nil {
		io.WriteString(w, "An error occurred when reading template")
		log.Println(err)
		return
	}
	tmpl.Execute(w, ServerInfo{RequestsPerHour: strconv.FormatInt(RateCounter.Rate(), 10), ResponseTime: LastResponse})
}

func processColorFlag(flag string, r *http.Request, validExceptions ...string) (valid bool, color string) {
	flagData := r.URL.Query().Get(flag)
	if flagData != "" {
		color, err := colors.Parse(flagData)
		if err == nil {
			return true, color.ToHEX().String()
		} else {
			color, err := colors.Parse("#" + flagData)
			if err == nil {
				return true, color.ToHEX().String()
			}
		}
	}
	for _, value := range validExceptions {
		if value == flagData {
			return true, flagData
		}
	}
	return false, ""
}

func widgetResponse(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	RateCounter.Incr(1)
	WidgetTemplate = "horizontal"
	widgetTemplate := r.URL.Query().Get("widgetTemplate")
	if widgetTemplate == "horizontal" || widgetTemplate == "vertical" || widgetTemplate == "compact" {
		WidgetTemplate = widgetTemplate
	}
	tmpl, err := template.ParseFiles("www/" + WidgetTemplate + ".html")
	if err != nil {
		io.WriteString(w, "An error occurred when reading template")
		log.Println(err)
		return
	}
	regex, err := regexp.Compile(`/widget/(?P<id>[0-9]+)`)
	if err != nil {
		io.WriteString(w, "An error occurred finding project id")
		log.Println(err)
		return
	}

	match := regex.FindStringSubmatch(r.URL.String())
	result := make(map[string]string)
	for i, name := range regex.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}

	projectID := result["id"]
	if projectID == "" {
		io.WriteString(w, "No or invalid project id provided")
		log.Println(err)
		return
	}

	cacheData, found := Cache.Get(projectID)
	if !found {
		project, err := getProjectData(projectID)
		if err != nil {
			io.WriteString(w, "An error occurred when loading curse data")
			log.Println(err)
			return
		}
		Cache.Set(projectID, project, cache.DefaultExpiration)
		cacheData = project
	}

	var projectData ProjectData

	projectData = cacheData.(ProjectData)

	//projectData.(ProjectData).SimulateDownloadCount = false

	//simulateDownloadCountParam := r.URL.Query().Get("simulateDownloadCount")
	//if simulateDownloadCountParam != "" {
	//	simBool, err := strconv.ParseBool(simulateDownloadCountParam)
	//	if err == nil {
	//		projectData.(*ProjectData).SimulateDownloadCount = simBool
	//	}
	//}

	DirectDownload = false
	directDownload := r.URL.Query().Get("directDownload")
	if directDownload != "" {
		directDlBool, err := strconv.ParseBool(directDownload)
		if err == nil {
			DirectDownload = directDlBool
		}
	}

	projectData.AccentColor = "#2c3e50"
	accentValid, accentColor := processColorFlag("accentColor", r)
	if accentValid {
		projectData.AccentColor = accentColor
	} else if projectData.ImageAccentColor != "" {
		projectData.AccentColor = projectData.ImageAccentColor
	}
	projectData.AccentColorHalfAlpha = projectData.AccentColor + "80"

	color, err := colors.Parse(projectData.AccentColor)
	if !color.IsDark() {
		projectData.ButtonTextColor = "black"
	} else {
		projectData.ButtonTextColor = "white"
	}

	projectData.NormalTextColor = "black"
	projectData.ShadowColor = "#888888"
	projectData.BackgroundColor = "transparent"
	darkTheme := r.URL.Query().Get("darkTheme")
	if darkTheme != "" {
		darkBool, err := strconv.ParseBool(darkTheme)
		if err == nil {
			if darkBool == true {
				projectData.NormalTextColor = "white"
				projectData.ShadowColor = "transparent"
				projectData.BackgroundColor = "#1B1B1B"
			}
		}
	}

	overrideButtonTextValid, overrideButtonTextColor := processColorFlag("overrideButtonTextColor", r)
	if overrideButtonTextValid {
		projectData.ButtonTextColor = overrideButtonTextColor
	}

	normalTextValid, normalTextColor := processColorFlag("normalTextColor", r)
	if normalTextValid {
		projectData.NormalTextColor = normalTextColor
	}

	shadowValid, buttonShadowColor := processColorFlag("buttonShadowColor", r, "transparent")
	if shadowValid {
		projectData.ShadowColor = buttonShadowColor
	}

	backgroundValid, backgroundColor := processColorFlag("backgroundColor", r, "transparent")
	if backgroundValid {
		projectData.BackgroundColor = backgroundColor
	}

	tmpl.Execute(w, projectData)
	LastResponse = time.Since(startTime).String()
}

func getProjectData(projectID string) (ProjectData, error) {

	addonData := ProjectData{}

	addon, err := cav2.GetAddon(projectID)
	if err != nil {
		return addonData, err
	}

	if addon == nil {
		return addonData, errors.New("failed to load curse addondata")
	}

	addonData.AddonInfo = addon

	//Populate the extra fields I added to make things easier for the template
	for _, attachment := range addonData.AddonInfo.Attachments {
		if attachment.IsDefault {
			addonData.Thumbnail = attachment.URL
		}
	}
	addonData.DownloadCountPretty = humanize.Comma(int64(addonData.AddonInfo.DownloadCount))

	//monthlyDownloads, err := getMonthlyDownloads(strconv.Itoa(addonData.AddonInfo.ID), addonData.AddonInfo.GameID)

	latestFile := populateLatestVersion(addonData)
	fildID := strconv.Itoa(latestFile.ProjectFileID)
	addonData.DownloadVersion = latestFile.GameVersion
	if DirectDownload {
		addonData.DownloadURL = "https://minecraft.curseforge.com/projects/" + projectID + "/files/" + fildID
	} else {
		addonData.DownloadURL = "https://minecraft.curseforge.com/projects/" + projectID + "/files/" + fildID + "/download"
	}
	addonData.ProjectURL = "https://minecraft.curseforge.com/projects/" + projectID

	//	if err == nil && monthlyDownloads > 0 {
	//		addonData.DownloadsPerSecond = monthlyDownloads / (30 * 24 * 60 * 60)
	//	} else {
	//No need to fail if this fails
	//	log.Println("Failed to get download history for " + projectID)
	//	log.Println(err)
	addonData.DownloadsPerSecond = 0
	//	}

	url := addonData.Thumbnail

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	checkErr := func(err error) {
		if err != nil {
			panic(err)
		}
	}

	img, _, err := image.Decode(resp.Body)
	checkErr(err)

	palette, err := vibrant.NewPaletteFromImage(img)
	checkErr(err)

	vibrantColor := palette.ExtractAwesome()["Vibrant"]
	if err == nil && vibrantColor != nil {
		addonData.ImageAccentColor = vibrantColor.Color.RGBHex()
	}

	return addonData, nil
}

func populateLatestVersion(projectData ProjectData) cav2.AddonGameVersion {
	var latestFile cav2.AddonGameVersion
	for _, file := range projectData.AddonInfo.GameVersionLatestFiles {
		gameVersion, err := semver.Make(file.GameVersion)
		if err != nil {
			//This wont work for things such as snapshots or other things that have stupid versions
			continue
		}
		//Checks to see if the game version set is valid, if not we assume its newer than the current version
		if latestFile.GameVersion == "" {
			latestFile = file
			continue
		}
		latestFileGameVersion, err := semver.Make(latestFile.GameVersion)
		if err != nil {
			continue
		}
		if gameVersion.Compare(latestFileGameVersion) == 1 {
			if isMostPromotedFile(projectData, file) {
				latestFile = file
			}
		}
	}
	return latestFile
}

//Checks the file to see if it is the best file for the job, ie a beta file will return true when if no release file is present but an alpha is.
func isMostPromotedFile(data ProjectData, testFile cav2.AddonGameVersion) bool {
	isBest := true
	for _, file := range data.AddonInfo.GameVersionLatestFiles {
		if file.GameVersion == testFile.GameVersion {
			if file.FileType < testFile.FileType {
				isBest = false
				break
			}
		}
	}
	return isBest
}

func getMonthlyDownloads(projectID string, gameID int) (float64, error) {
	var historyData map[string]float64
	if x, found := HistoryCache.Get(strconv.Itoa(gameID)); found {
		historyData = x.(map[string]float64)
	} else {
		historyBytes, err := goutils.Download("https://cursemeta.dries007.net/api/v2/history/downloads/" + strconv.Itoa(gameID) + "/monthly")
		if err != nil {
			return 0, err
		}
		var downloadMap = make(map[string]float64)
		err = json.Unmarshal(historyBytes, &downloadMap)
		if err != nil {
			return 0, err
		}
		HistoryCache.Set(strconv.Itoa(gameID), downloadMap, cache.DefaultExpiration)
		historyData = downloadMap
	}
	return historyData[projectID], nil
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func openLogFile(logfile string) {
	if logfile != "" {
		lf, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)

		if err != nil {
			log.Fatal("OpenLogfile: os.OpenFile:", err)
		}

		log.SetOutput(lf)
	}
}

type ServerInfo struct {
	RequestsPerHour string
	ResponseTime    string
}

//Made with https://mholt.github.io/json-to-go/
type ProjectData struct {
	Thumbnail             string //Not in json, moved here to make things easier
	DownloadCountPretty   string //This is a nice looking download count
	DownloadsPerSecond    float64
	SimulateDownloadCount bool
	DownloadVersion       string
	DownloadURL           string
	ProjectURL            string
	AccentColor           string
	AccentColorHalfAlpha  string
	ImageAccentColor      string
	ButtonTextColor       string
	NormalTextColor       string
	ShadowColor           string
	BackgroundColor       string

	AddonInfo *cav2.Addon
}
