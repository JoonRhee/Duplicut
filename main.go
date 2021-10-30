package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var routineCount int
var bufferSize int
var rootsList binding.StringList
var mainApp fyne.App
var subWindow fyne.Window
var collisionGroups [][]string

//Validator for the settings entries
func entryValidator(s string) error {
	i, err := strconv.Atoi(s)
	if i < 1 {
		return errors.New("too low")
	}
	return err
}

//Remove a string from string slice
func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func validateDir(x string, y string) error {
	err := errors.New(y + "\nis, or subdirectory of, already added folder\n" + x)
	xsplit := strings.Split(x, "/")
	ysplit := strings.Split(y, "/")

	if len(xsplit) > len(ysplit) {
		xsplit, ysplit = ysplit, xsplit
		err = errors.New(y + "\nis a root folder of\n" + x)
	}

	for i := range xsplit {
		if xsplit[i] != ysplit[i] {
			return nil
		}
	}
	return err
}

//set subWindow to settings screen
func settings() {
	//if there is a sub window open already, don't open another window
	if subWindow != nil {
		subWindow.RequestFocus()
		return
	}

	//create new sub window and set it up
	settingsWindow := mainApp.NewWindow("Duplicut - Settings")
	subWindow = settingsWindow
	settingsWindow.Resize(fyne.NewSize(400, 175))
	settingsWindow.SetCloseIntercept(func() { settingsWindow.Close(); subWindow = nil })
	settingsWindow.CenterOnScreen()

	//set up window title and done button
	mainLabel := widget.NewLabel("Settings")
	doneButton := widget.NewButton("Done", func() { settingsWindow.Close(); subWindow = nil })

	//set up widgets for goroutine count settings
	routineCountLabel := widget.NewLabel("# Of Goroutines:")
	routineCountEntry := widget.NewEntry()
	routineCountEntry.SetPlaceHolder(strconv.Itoa(routineCount))
	routineCountEntry.Validator = entryValidator
	routineCountEntry.OnSubmitted = func(s string) {
		if routineCountEntry.Validate() == nil {
			routineCount, _ = strconv.Atoi(routineCountEntry.Text)
			dialog.NewInformation("Saved!", "# of Goroutines set to "+routineCountEntry.Text, settingsWindow).Show()
		}
	}
	routineCountSave := widget.NewButtonWithIcon("", theme.DocumentSaveIcon(), func() {
		if routineCountEntry.Validate() == nil {
			routineCount, _ = strconv.Atoi(routineCountEntry.Text)
			dialog.NewInformation("Saved!", "# of Goroutines set to "+routineCountEntry.Text, settingsWindow).Show()
		}
	})

	//set up widgets for buffer size settings
	bufferSizeLabel := widget.NewLabel("Buffer Size(Bytes):")
	bufferSizeEntry := widget.NewEntry()
	bufferSizeEntry.SetPlaceHolder(strconv.Itoa(bufferSize))
	bufferSizeEntry.Validator = entryValidator
	bufferSizeEntry.OnSubmitted = func(s string) {
		if bufferSizeEntry.Validate() == nil {
			bufferSize, _ = strconv.Atoi(bufferSizeEntry.Text)
			dialog.NewInformation("Saved!", "Buffer Size set to "+bufferSizeEntry.Text, settingsWindow).Show()
		}
	}
	bufferSizeSave := widget.NewButtonWithIcon("", theme.DocumentSaveIcon(), func() {
		if bufferSizeEntry.Validate() == nil {
			bufferSize, _ = strconv.Atoi(bufferSizeEntry.Text)
			dialog.NewInformation("Saved!", "Buffer Size set to "+bufferSizeEntry.Text, settingsWindow).Show()
		}
	})

	//set window's content to setting and show
	settingsWindow.SetContent(container.NewVBox(
		container.NewHBox(layout.NewSpacer(), mainLabel, layout.NewSpacer(), doneButton),
		layout.NewSpacer(),
		container.NewBorder(nil, nil, routineCountLabel, routineCountSave, routineCountEntry),
		layout.NewSpacer(),
		container.NewBorder(nil, nil, bufferSizeLabel, bufferSizeSave, bufferSizeEntry),
		layout.NewSpacer(),
	))
	settingsWindow.Show()
}

//set subWindow to folder adding screen
func addFolder() {
	//if there is a sub window open already, don't open another window
	if subWindow != nil {
		subWindow.RequestFocus()
		return
	}

	//create new sub window and set it up
	addWindow := mainApp.NewWindow("Duplicut - Add Folder")
	subWindow = addWindow
	addWindow.Resize(fyne.NewSize(400, 600))
	addWindow.SetCloseIntercept(func() { addWindow.Close(); subWindow = nil })
	addWindow.CenterOnScreen()

	//set up window title, add button, done button
	mainLabel := widget.NewLabel("Folder List")
	addButton := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				rootsSlice, _ := rootsList.Get()
				for _, root := range rootsSlice {
					err := validateDir(root, uri.Path())
					if err != nil {
						dialog.ShowError(err, addWindow)
						return
					}
				}
				rootsList.Append(uri.Path())
			}
		}, addWindow)
	})
	doneButton := widget.NewButton("Done", func() { addWindow.Close(); subWindow = nil })

	//set up list of directories that has been added
	folderList := widget.NewListWithData(
		rootsList,
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewHyperlink("Template", nil),
				layout.NewSpacer(),
				widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {}))
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			ct := o.(*fyne.Container)
			link := ct.Objects[0].(*widget.Hyperlink)
			str, _ := i.(binding.String).Get()
			link.SetText(str)
			link.SetURLFromString(str)
			ct.Objects[2] = widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				premove, _ := rootsList.Get()
				rootsList.Set(remove(premove, link.Text))
			})
		},
	)

	//set window's content to addFolder and show
	addWindow.SetContent(container.NewBorder(
		container.NewHBox(addButton, layout.NewSpacer(), mainLabel, layout.NewSpacer(), doneButton),
		nil,
		nil,
		nil,
		folderList,
	))
	addWindow.Show()
}

//set subWindow to collision screen
func showCollisions() {
	//create new sub window and set it up
	colWindow := mainApp.NewWindow("Duplicut - Collisions")
	subWindow = colWindow
	colWindow.SetCloseIntercept(func() { colWindow.Close(); subWindow = nil })
	colWindow.CenterOnScreen()
	closeButton := widget.NewButton("Done", func() {
		colWindow.Close()
		subWindow = nil
	})

	//if there is no collision, open small notice instead
	if len(collisionGroups) < 1 {
		colWindow.SetContent(container.NewHBox(widget.NewLabel("No Collision!"), closeButton))
		colWindow.Show()
		return
	}

	//resize the window to fit collision group list
	colWindow.Resize(fyne.NewSize(400, 600))
	groupIndex := 0

	//set up title text
	mainLabelString := binding.NewString()
	mainLabel := widget.NewLabelWithData(mainLabelString)
	mainLabelString.Set("Collision Group " + strconv.Itoa(groupIndex+1) + "/" + strconv.Itoa(len(collisionGroups)))

	//set up collision list
	currentList := binding.NewStringList()
	currentList.Set(collisionGroups[groupIndex])
	collisionList := widget.NewListWithData(currentList,
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewButtonWithIcon("", theme.FolderIcon(), func() {}),
				widget.NewHyperlink("Template", nil),
				layout.NewSpacer(),
				widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {}))
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			ct := o.(*fyne.Container)
			link := ct.Objects[1].(*widget.Hyperlink)
			str, _ := i.(binding.String).Get()
			link.SetText(str)
			link.SetURLFromString(str)

			ct.Objects[0] = widget.NewButtonWithIcon("", theme.FolderIcon(), func() {
				u, _ := url.Parse(str[:strings.LastIndex(str, "\\")])
				mainApp.OpenURL(u)
			})

			ct.Objects[3] = widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				dialog.ShowConfirm(
					"Duplicut - Confirm Delete",
					"Are you sure you want to permanently delete \n"+str+"?",
					func(y bool) {
						if !y {
							return
						}
						err := os.Remove(str)

						if err != nil {
							dialog.ShowError(errors.New("failed to delete \n"+str+"\n"+err.Error()), colWindow)
							return
						}
						premove, _ := currentList.Get()
						removed := remove(premove, link.Text)
						currentList.Set(removed)
						collisionGroups[groupIndex] = removed
					},
					colWindow,
				)
			})

			ct.Objects[0].Refresh()
			ct.Objects[1].Refresh()
			ct.Objects[2].Refresh()
			ct.Objects[3].Refresh()
		},
	)

	//set up navigation buttons
	var backButton *widget.Button
	var nextButton *widget.Button
	backButton = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		groupIndex--
		currentList.Set(collisionGroups[groupIndex])
		mainLabelString.Set("Collision Group " + strconv.Itoa(groupIndex+1) + "/" + strconv.Itoa(len(collisionGroups)))
		mainLabel.Refresh()
		collisionList.Refresh()
		if groupIndex >= 0 {
			nextButton.Enable()
		}
		if groupIndex <= 0 {
			backButton.Disable()
		}

	})
	nextButton = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		groupIndex++
		currentList.Set(collisionGroups[groupIndex])
		mainLabelString.Set("Collision Group " + strconv.Itoa(groupIndex+1) + "/" + strconv.Itoa(len(collisionGroups)))
		mainLabel.Refresh()
		collisionList.Refresh()
		if groupIndex < len(collisionGroups) {
			backButton.Enable()
		}
		if groupIndex >= len(collisionGroups)-1 {
			nextButton.Disable()
		}
	})
	if groupIndex <= 0 {
		backButton.Disable()
	}
	if groupIndex >= len(collisionGroups)-1 {
		nextButton.Disable()
	}

	//set sub window's content to collision groups screen and show
	colWindow.SetContent(container.NewBorder(
		container.NewHBox(backButton, layout.NewSpacer(), mainLabel, layout.NewSpacer(), nextButton),
		container.NewHBox(layout.NewSpacer(), closeButton),
		nil,
		nil,
		collisionList,
	))
	colWindow.Show()
}

//set subWindow to search screen and search for collisions. when search is completed, run showCollisions
func search() {
	//if there is a sub window open already, don't open another window
	if subWindow != nil {
		subWindow.RequestFocus()
		return
	}

	//Setup window
	searchWindow := mainApp.NewWindow("Searching...")
	subWindow = searchWindow
	stopped := false
	searchWindow.SetCloseIntercept(func() { searchWindow.Close(); stopped = true; subWindow = nil })
	searchWindow.CenterOnScreen()
	searchWindow.Resize(fyne.NewSize(400, 40))

	//Setup progBar widget
	progBarData := binding.NewFloat()
	progBar := widget.NewProgressBarWithData(progBarData)

	//Setup status label widget
	status := widget.NewLabel("")
	status.SetText(fmt.Sprintf("%-30s", "Getting file paths..."))

	//Setup cancel Button widget
	cancelButton := widget.NewButtonWithIcon("Quit", theme.CancelIcon(), func() {
		searchWindow.Close()
		stopped = true
		subWindow = nil
	})

	//Generate and set window content
	searchContent := container.NewBorder(nil, nil, status, cancelButton, progBar)
	searchWindow.SetContent(searchContent)
	searchWindow.Show()
	time.Sleep(time.Millisecond * 500)

	//Start Process
	collisionGroups = [][]string{}

	//run as goroutine so pause button is accessible
	go func() {
		//Gather all file path that is inside the given directories
		st := time.Now()
		paths := []string{}
		roots, _ := rootsList.Get()
		for _, root := range roots {
			filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
				if err == nil && !info.IsDir() {
					paths = append(paths, path)
				}
				return nil
			})
		}

		//Setup variables for concurrent duplicate file finding
		collisionMap := make(map[string][]string)
		var wg sync.WaitGroup
		guard := make(chan struct{}, routineCount)
		mutex := &sync.Mutex{}

		//Data for displaying
		finished := 0
		total := len(paths)
		errCount := 0

		//Go through all the paths
		for _, path := range paths {

			//Stop generating goroutines if cancel button is pressed
			if stopped {
				break
			}

			//Limit goroutine generation
			guard <- struct{}{}
			wg.Add(1)

			//Start goroutine to parse files
			go func(path string) {
				defer func() { wg.Done(); <-guard }()

				//Open file from given path
				f, openErr := os.Open(path)
				if openErr != nil {
					mutex.Lock()
					errCount++
					mutex.Unlock()
					return
				}

				//Setup buffer for file reading
				buffer := make([]byte, bufferSize)
				h := sha256.New()

				//Reading file
				for {
					if stopped {
						return
					}
					bytesRead, readErr := f.Read(buffer)
					if readErr != nil {
						if readErr != io.EOF {
							mutex.Lock()
							errCount++
							mutex.Unlock()
							return
						}
						h.Write(buffer[:bytesRead])
						break
					}
					h.Write(buffer[:bytesRead])
				}

				//Generate fileHash
				fileHash := hex.EncodeToString(h.Sum(nil))

				//Append the file path to collision map
				mutex.Lock()
				collisionMap[fileHash] = append(collisionMap[fileHash], path)
				finished++
				progBarData.Set(float64(finished+errCount) / float64(total))
				status.SetText(fmt.Sprintf("%-30s", strconv.Itoa(finished)+"/"+strconv.Itoa(total)+"(errors: "+strconv.Itoa(errCount)+")"))
				mutex.Unlock()
			}(path)
		}
		wg.Wait()
		et := time.Now()

		//if stopped, quit
		if stopped {
			collisionGroups = [][]string{}
			return
		}

		//find all collisions
		for hashKey := range collisionMap {
			if len(collisionMap[hashKey]) > 1 {
				collisionGroups = append(collisionGroups, collisionMap[hashKey])
			}
		}

		//send end notification
		mainApp.SendNotification(fyne.NewNotification("Duplicut: Search Complete!", "Detected "+strconv.Itoa(len(collisionGroups))+" collision groups in "+et.Sub(st).String()))

		searchWindow.Close()
		subWindow = nil
		showCollisions()
	}()
}

//run main window
func main() {
	//set font, theme, and window icon
	os.Setenv("FYNE_THEME", `dark`)
	windowIcon, _ := fyne.LoadResourceFromPath(`.\Icon.png`)

	//run main app
	mainApp = app.New()
	mainApp.SetIcon(windowIcon)

	//setup mainWindow
	mainWindow := mainApp.NewWindow("Duplicut")
	mainWindow.Resize(fyne.NewSize(400, 200))
	mainWindow.SetIcon(windowIcon)
	mainWindow.CenterOnScreen()

	//setup default settings
	routineCount = 50
	bufferSize = 32768
	rootsList = binding.NewStringList()

	//set title image
	image := canvas.NewImageFromFile(`.\Title.png`)
	image.SetMinSize(fyne.NewSize(256, 128))

	//set all buttons
	settingsButton := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), settings)
	folderButton := widget.NewButtonWithIcon("Add Folder", theme.FolderNewIcon(), addFolder)
	searchButton := widget.NewButtonWithIcon("Start", theme.MediaPlayIcon(), search)

	//set contents to main window and run
	mainWindow.SetContent(
		container.NewVBox(
			layout.NewSpacer(),
			container.NewHBox(layout.NewSpacer(), image, layout.NewSpacer()),
			layout.NewSpacer(),
			container.NewHBox(layout.NewSpacer(), settingsButton, layout.NewSpacer(), folderButton, layout.NewSpacer(), searchButton, layout.NewSpacer()),
			layout.NewSpacer(),
		))
	mainWindow.ShowAndRun()
}
