package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
)

var (
	pkgPath   string
	version   string
	appName   string
	addonPath string
	xmlPaths  map[string]string
)

func init() {
	// File & directory paths
	// Assume the installer is run from the project root
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	pkgPath = filepath.Join(cwd, "wpsjs")

	// Read version from version.js
	versionJS, err := ioutil.ReadFile(filepath.Join(pkgPath, "version.js"))
	if err != nil {
		panic(fmt.Sprintf("Failed to read version.js: %v", err))
	}
	re := regexp.MustCompile(`=\s*["'](.*)["']`)
	matches := re.FindStringSubmatch(string(versionJS))
	if len(matches) < 2 {
		panic("Could not find version in version.js")
	}
	version = matches[1]
	appName = fmt.Sprintf("wps-zotero_%s", version)

	// Platform-specific addon path
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			panic("APPDATA environment variable is not set")
		}
		addonPath = filepath.Join(appData, "kingsoft", "wps", "jsaddons")
	} else { // posix
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		addonPath = filepath.Join(homeDir, ".local", "share", "Kingsoft", "wps", "jsaddons")
	}

	xmlPaths = map[string]string{
		"jsplugins":   filepath.Join(addonPath, "jsplugins.xml"),
		"publish":     filepath.Join(addonPath, "publish.xml"),
		"authwebsite": filepath.Join(addonPath, "authwebsite.xml"),
	}
}

func uninstall() {
	if runtime.GOOS == "windows" {
		fmt.Println("Trying to quit proxy server if it's currently listening...")
		exec.Command("taskkill", "/F", "/IM", "proxy.exe").Run()
	}
	fmt.Println("Uninstalling previous installations if there is any ...")

	_, err := os.Stat(addonPath)
	if os.IsNotExist(err) {
		return // Nothing to uninstall
	}

	// Remove wps-zotero directories
	entries, err := ioutil.ReadDir(addonPath)
	if err != nil {
		fmt.Printf("Could not read addon path %s: %v\n", addonPath, err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(entry.Name(), "wps-zotero") {
			dirToRemove := filepath.Join(addonPath, entry.Name())
			fmt.Printf("Removing %s\n", dirToRemove)
			err := os.RemoveAll(dirToRemove)
			if err != nil {
				fmt.Printf("Failed to remove directory %s: %v\n", dirToRemove, err)
			}
		}
	}

	// Remove records from XML files
	re := regexp.MustCompile(`(?m)^[\s\t]*<.*wps-zotero.* />\s*\r?\n?`)
	for _, fp := range xmlPaths {
		if _, err := os.Stat(fp); os.IsNotExist(err) {
			continue
		}
		content, err := ioutil.ReadFile(fp)
		if err != nil {
			fmt.Printf("Could not read %s: %v\n", fp, err)
			continue
		}
		newContent := re.ReplaceAll(content, []byte{})
		if len(newContent) != len(content) {
			fmt.Printf("Removing record from %s\n", fp)
			err = ioutil.WriteFile(fp, newContent, 0644)
			if err != nil {
				fmt.Printf("Failed to write to %s: %v\n", fp, err)
			}
		}
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		return ioutil.WriteFile(dstPath, data, info.Mode())
	})
}

func register(fp, tagName, record string) {
	content, err := ioutil.ReadFile(fp)
	if err != nil {
		fmt.Printf("Failed to read %s for registration: %v\n", fp, err)
		return
	}

	strContent := string(content)
	tag := fmt.Sprintf("</%s>", tagName)
	pos := strings.Index(strContent, tag)
	if pos == -1 {
		fmt.Printf("Tag %s not found in %s\n", tag, fp)
		// If tag is not found, maybe append it? For now, just log.
		return
	}

	newContent := strContent[:pos] + record + "\n" + strContent[pos:]
	err = ioutil.WriteFile(fp, []byte(newContent), 0644)
	if err != nil {
		fmt.Printf("Failed to write registration to %s: %v\n", fp, err)
	}
}

func fixZoteroPrefs() {
	if runtime.GOOS != "windows" {
		return
	}
	fmt.Println("Change zotero preference to alleviate the problem of Zotero window not showing in front.")

	appData := os.Getenv("APPDATA")
	profilesPath := filepath.Join(appData, "Zotero", "Zotero", "Profiles")

	profiles, err := ioutil.ReadDir(profilesPath)
	if err != nil {
		fmt.Printf("Could not read Zotero profiles directory: %v\n", err)
		return
	}

	for _, profile := range profiles {
		if profile.IsDir() && strings.HasSuffix(profile.Name(), ".default") {
			prefsFile := filepath.Join(profilesPath, profile.Name(), "prefs.js")
			if _, err := os.Stat(prefsFile); err == nil {
				content, err := ioutil.ReadFile(prefsFile)
				if err != nil {
					continue
				}
				strContent := string(content)
				prefKey := "extensions.zotero.integration.keepAddCitationDialogRaised"
				if strings.Contains(strContent, prefKey) {
					strContent = strings.Replace(strContent,
						fmt.Sprintf("user_pref(\"%s\", false)", prefKey),
						fmt.Sprintf("user_pref(\"%s\", true);", prefKey),
						-1)
				} else {
					strContent += fmt.Sprintf("\nuser_pref(\"%s\", true);\n", prefKey)
				}
				ioutil.WriteFile(prefsFile, []byte(strContent), 0644)
			}
		}
	}
}

func startProxy() {
	if runtime.GOOS == "windows" {
		fmt.Println("Starting proxy server as a detached process...")
		cmd := exec.Command("./proxy.exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: 0x08000000, // DETACHED_PROCESS
		}
		if err := cmd.Start(); err != nil {
			fmt.Printf("Failed to start proxy.exe: %v\n", err)
			fmt.Println("Please start it manually.")
		}
	}
}

func main() {
	// Uninstall existing installation
	uninstall()
	if len(os.Args) > 1 && os.Args[1] == "-u" {
		fmt.Println("Uninstallation complete.")
		return
	}

	// Begin installation
	fmt.Println("Installing")

	// Create necessary directory and files
	if err := os.MkdirAll(addonPath, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create addon path: %v", err))
	}

	if _, err := os.Stat(xmlPaths["jsplugins"]); os.IsNotExist(err) {
		ioutil.WriteFile(xmlPaths["jsplugins"], []byte("<jsplugins>\n</jsplugins>"), 0644)
	}
	if _, err := os.Stat(xmlPaths["publish"]); os.IsNotExist(err) {
		ioutil.WriteFile(xmlPaths["publish"], []byte(`<?xml version="1.0" encoding="UTF-8"?>`+"\n"+`<jsplugins>\n</jsplugins>`), 0644)
	}
	if _, err := os.Stat(xmlPaths["authwebsite"]); os.IsNotExist(err) {
		ioutil.WriteFile(xmlPaths["authwebsite"], []byte(`<?xml version="1.0" encoding="UTF-8"?>`+"\n"+`<websites>\n</websites>`), 0644)
	}

	// Copy to jsaddons
	destPath := filepath.Join(addonPath, appName)
	fmt.Printf("Copying plugin files to %s\n", destPath)
	if err := copyDir(pkgPath, destPath); err != nil {
		panic(fmt.Sprintf("Failed to copy plugin files: %v", err))
	}

	// Write records to XML files
	rec1 := fmt.Sprintf(`<jsplugin name="wps-zotero" type="wps" url="http://127.0.0.1:3889/" version="%s"/>`, version)
	register(xmlPaths["jsplugins"], "jsplugins", rec1)

	rec2 := fmt.Sprintf(`<jsplugin url="http://127.0.0.1:3889/" type="wps" enable="enable_dev" install="null" version="%s" name="wps-zotero"/>`, version)
	register(xmlPaths["publish"], "jsplugins", rec2)

	rec3 := `<website origin="null" name="wps-zotero" status="enable"/>`
	register(xmlPaths["authwebsite"], "websites", rec3)

	// Zotero preference fix
	fixZoteroPrefs()

	startProxy()

	fmt.Println("All done, enjoy!")
	fmt.Println("(run with -u flag to uninstall)")
}
