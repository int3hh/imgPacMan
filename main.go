package main

import (
	"errors"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/fsnotify/fsnotify"
	"willnorris.com/go/imageproxy"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Config folderz `yaml:"Folders"`
}
type folderz []Folder
type Folder struct {
	Path   string `yaml:"path"`
	Size   string `yaml:"size"`
	Thumb  string `yaml:"thumb"`
	ImageW int
	ImageH int
	ThumbW int
	ThumbH int
}

var allowedExtensions = [...]string{"jpeg", "jpg", "png"}
var processChan chan string
var last string
var folders Config

func getFiles(folder string) {
	filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		for _, ext := range allowedExtensions {
			if strings.HasSuffix(path, ext) {
				if !strings.Contains(path, "_thumb") {
					processChan <- path
					break
				}
			}
		}
		return nil
	})
}

func parseSize(str string) (int, int) {
	size := strings.Split(str, "x")
	width, err := strconv.Atoi(size[0])
	height, err2 := strconv.Atoi(size[1])
	if err == nil && err2 == nil {
		return width, height
	}

	return 0, 0
}

func getFolderFromFile(path string) (Folder, error) {
	for _, folder := range folders.Config {
		if strings.HasPrefix(path, folder.Path) {
			return folder, nil
		}
	}
	return Folder{}, errors.New("Folder not found")
}

func getThumbName(fname string) string {
	if idx := strings.LastIndex(fname, "."); idx > 0 {
		return fname[:idx] + "_thumb" + fname[idx:]
	}
	return fname
}

func main() {
	processChan = make(chan string)
	watch, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			event, ok := <-watch.Events
			if ok {
				if event.Op&fsnotify.Write == fsnotify.Write && last != event.Name {
					processChan <- event.Name
					last = event.Name
				}
			}
		}
	}()

	defer watch.Close()
	cfgData, err := ioutil.ReadFile("config.yaml")
	if err == nil {
		err = yaml.Unmarshal(cfgData, &folders)
		if err == nil {
			go func() {
				for {
					fname := <-processChan
					if strings.Contains(fname, "_thumb") {
						continue
					}
					log.Println("Processing ... " + fname)

					folder, err := getFolderFromFile(fname)
					if err != nil {
						continue
					}
					thumb := getThumbName(fname)
					_, err = os.Stat(thumb)

					if err != nil && os.IsNotExist(err) {
						fsource, err := ioutil.ReadFile(fname)
						if err == nil {
							fdest, err := imageproxy.Transform(fsource, imageproxy.Options{Width: float64(folder.ImageW), Height: float64(folder.ImageH)})
							if err == nil {
								log.Println("Gen resized image ... " + fname)
								ioutil.WriteFile(fname, fdest, 0644)
							}

							if folder.ThumbH > 0 && folder.ThumbW > 0 {
								fsource, err := ioutil.ReadFile(fname)
								if err == nil {
									fdest, err := imageproxy.Transform(fsource, imageproxy.Options{Width: float64(folder.ThumbW), Height: float64(folder.ThumbH)})
									if err == nil {
										log.Println("Gen thumb ... " + thumb)
										ioutil.WriteFile(thumb, fdest, 0644)
									}
								}
							}
						}
					}
				}
			}()

			for k, folder := range folders.Config {
				log.Println("Processing folder ... " + folder.Path)
				if _, err = os.Stat(folder.Path); os.IsNotExist(err) {
					log.Panic("Folder does not exist " + folder.Path)
				}

				hasOne := false

				if len(folder.Size) > 0 {
					folders.Config[k].ImageW, folders.Config[k].ImageH = parseSize(folder.Size)
					if folders.Config[k].ImageW+folders.Config[k].ImageH == 0 {
						log.Panic("Invalid size for: " + folder.Path)
					}
					hasOne = true
				}

				if len(folder.Thumb) > 0 {
					folders.Config[k].ThumbW, folders.Config[k].ThumbH = parseSize(folder.Thumb)
					if folders.Config[k].ThumbW+folders.Config[k].ThumbH == 0 {
						log.Panic("Invalid size for: " + folder.Path)
					}
					hasOne = true
				}

				if !hasOne {
					log.Panic("Invalid rule for: " + folder.Path)
				}
			}

			for _, folder := range folders.Config {
				getFiles(folder.Path)
				err = watch.Add(folder.Path)
				if err != nil {
					log.Panic(err)
				}
			}

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
			log.Println("Waiting ...")
			<-sigs
			log.Println("Terminating...")

		} else {
			log.Panic("Config file is invalid")
		}
	} else {
		log.Panic("Unable to read config file")
	}

}
