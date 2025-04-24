package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/gen2brain/iup-go/iup"
	"github.com/go-ldap/ldap/v3"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// LDAPConfig stores connection configuration
type LDAPConfig struct {
	Host     string
	Port     string
	BindDN   string
	Password string
	BaseDN   string
}

// LDIFEntry represents a single LDAP entry from the LDIF file
type LDIFEntry struct {
	DN              string
	ObjectClass     string
	SN              string
	CN              string
	OU              string
	Title           string
	Mail            string
	GivenName       string
	Initials        string
	TelephoneNumber string
	L               string
	PostalAddress   string
	O               string
}

// ProgressDialog manages the progress window
type ProgressDialog struct {
	Window    *gtk.Dialog
	Progress  *gtk.ProgressBar
	Label     *gtk.Label
	CancelBtn *gtk.Button
	Canceled  bool
	Mutex     sync.Mutex
}

var (
	config     LDAPConfig
	counterDlg iup.Ihandle
)

func main() {
	// Initialize GTK
	gtk.Init(nil)
	os.Setenv("GDK_BACKEND", "x11")

	// Create and show main window
	win, err := createMainWindow()
	if err != nil {
		log.Fatal("Failed to create main window:", err)
	}

	win.ShowAll()

	// Start the GTK main loop
	gtk.Main()
}

func createMainWindow() (*gtk.Window, error) {
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return nil, err
	}

	win.SetTitle("LDAP Data Loader")
	win.SetDefaultSize(450, 350)
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	grid, err := gtk.GridNew()
	if err != nil {
		return nil, err
	}
	grid.SetOrientation(gtk.ORIENTATION_VERTICAL)
	grid.SetBorderWidth(10)
	grid.SetRowSpacing(5)
	grid.SetColumnSpacing(5)

	// Connection settings
	connLabel, err := gtk.LabelNew("LDAP Connection Settings")
	if err != nil {
		return nil, err
	}
	grid.Attach(connLabel, 0, 0, 2, 1)

	// Host
	hostLabel, err := gtk.LabelNew("Host:")
	if err != nil {
		return nil, err
	}
	hostEntry, err := gtk.EntryNew()
	if err != nil {
		return nil, err
	}
	hostEntry.SetPlaceholderText("ldap.example.com")
	grid.Attach(hostLabel, 0, 1, 1, 1)
	grid.Attach(hostEntry, 1, 1, 1, 1)
	hostEntry.SetText("localhost")

	// Port
	portLabel, err := gtk.LabelNew("Port:")
	if err != nil {
		return nil, err
	}
	portEntry, err := gtk.EntryNew()
	if err != nil {
		return nil, err
	}
	portEntry.SetText("389")
	grid.Attach(portLabel, 0, 2, 1, 1)
	grid.Attach(portEntry, 1, 2, 1, 1)

	// Bind DN
	bindDNLabel, err := gtk.LabelNew("Bind DN:")
	if err != nil {
		return nil, err
	}
	bindDNEntry, err := gtk.EntryNew()
	if err != nil {
		return nil, err
	}
	bindDNEntry.SetPlaceholderText("cn=admin,dc=example,dc=com")
	bindDNEntry.SetText("cn=admin,dc=mail,dc=local")
	grid.Attach(bindDNLabel, 0, 3, 1, 1)
	grid.Attach(bindDNEntry, 1, 3, 1, 1)

	// Password
	passLabel, err := gtk.LabelNew("Password:")
	if err != nil {
		return nil, err
	}
	passEntry, err := gtk.EntryNew()
	if err != nil {
		return nil, err
	}
	passEntry.SetText("123456")
	passEntry.SetVisibility(false)
	grid.Attach(passLabel, 0, 4, 1, 1)
	grid.Attach(passEntry, 1, 4, 1, 1)

	// Base DN
	baseDNLabel, err := gtk.LabelNew("Base DN:")
	if err != nil {
		return nil, err
	}
	baseDNEntry, err := gtk.EntryNew()
	if err != nil {
		return nil, err
	}
	baseDNEntry.SetPlaceholderText("dc=example,dc=com")
	baseDNEntry.SetText("dc=mail,dc=local")
	grid.Attach(baseDNLabel, 0, 5, 1, 1)
	grid.Attach(baseDNEntry, 1, 5, 1, 1)

	// File selection
	fileLabel, err := gtk.LabelNew("LDIF File:")
	if err != nil {
		return nil, err
	}
	fileEntry, err := gtk.EntryNew()
	if err != nil {
		return nil, err
	}
	fileEntry.SetEditable(false)
	fileBtn, err := gtk.ButtonNewWithLabel("Select File")
	if err != nil {
		return nil, err
	}
	fileBtn.Connect("clicked", func() {
		fileChooser, err := gtk.FileChooserDialogNewWith2Buttons(
			"Select LDIF File",
			win,
			gtk.FILE_CHOOSER_ACTION_OPEN,
			"Cancel",
			gtk.RESPONSE_CANCEL,
			"Open",
			gtk.RESPONSE_ACCEPT,
		)
		if err != nil {
			log.Println("Error creating file chooser:", err)
			return
		}
		defer fileChooser.Destroy()

		filter, err := gtk.FileFilterNew()
		if err != nil {
			log.Println("Error creating file filter:", err)
			return
		}
		filter.SetName("LDIF Files")
		filter.AddPattern("*.ldif")
		fileChooser.AddFilter(filter)

		if fileChooser.Run() == gtk.RESPONSE_ACCEPT {
			filename := fileChooser.GetFilename()
			fileEntry.SetText(filename)
		}
	})
	grid.Attach(fileLabel, 0, 6, 1, 1)
	grid.Attach(fileEntry, 1, 6, 1, 1)
	grid.Attach(fileBtn, 2, 6, 1, 1)

	// OU selection
	ouLabel, err := gtk.LabelNew("Target OU:")
	if err != nil {
		return nil, err
	}
	ouCombo, err := gtk.ComboBoxTextNew()
	if err != nil {
		return nil, err
	}
	refreshBtn, err := gtk.ButtonNewWithLabel("Refresh OUs")
	if err != nil {
		return nil, err
	}

	refreshBtn.Connect("clicked", func() {
		//		var config LDAPConfig

		//		config.Host, err =
		//		config := LDAPConfig{
		config.Host, err = hostEntry.GetText()
		config.Port, err = portEntry.GetText()
		config.BindDN, err = bindDNEntry.GetText()
		config.Password, err = passEntry.GetText()
		config.BaseDN, err = baseDNEntry.GetText()
		//		}

		ous, err := getOUs(config)
		if err != nil {
			showErrorDialog(win, "Failed to get OUs: "+err.Error())
			return
		}

		ouCombo.RemoveAll()
		for _, ou := range ous {
			ouCombo.AppendText(ou)
		}
	})
	grid.Attach(ouLabel, 0, 7, 1, 1)
	grid.Attach(ouCombo, 1, 7, 1, 1)
	grid.Attach(refreshBtn, 2, 7, 1, 1)

	// Buttons
	btnBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 5)
	if err != nil {
		return nil, err
	}

	buildTreeBtn, err := gtk.ButtonNewWithLabel("Build Tree")
	if err != nil {
		return nil, err
	}
	buildTreeBtn.Connect("clicked", func() {
		filename, _ := fileEntry.GetText()
		if filename == "" {
			showErrorDialog(win, "Please select an LDIF file first")
			return
		}

		entries, err := parseLDIF(filename)
		if err != nil {
			showErrorDialog(win, "Failed to parse LDIF file: "+err.Error())
			return
		}
		showTreeWindow(win, buildOrgTree(entries), entries, config)
		//		showTreeWindow(win, buildOrgTree(entries), entries)
	})

	loadBtn, err := gtk.ButtonNewWithLabel("Load Data")
	if err != nil {
		return nil, err
	}
	loadBtn.Connect("clicked", func() {

		config.Host, err = hostEntry.GetText()
		config.Port, err = portEntry.GetText()
		config.BindDN, err = bindDNEntry.GetText()
		config.Password, err = passEntry.GetText()
		config.BaseDN, err = baseDNEntry.GetText()

		targetOU := ouCombo.GetActiveText()
		if targetOU == "" {
			showErrorDialog(win, "Please select target OU")
			return
		}

		filename, _ := fileEntry.GetText()
		if filename == "" {
			showErrorDialog(win, "Please select an LDIF file first")
			return
		}

		entries, err := parseLDIF(filename)
		if err != nil {
			showErrorDialog(win, "Failed to parse LDIF file: "+err.Error())
			return
		}
		if len(entries) == 0 {
		}

		go func() {
			progressDialog := createProgressDialog(win, "Loading Data", "Deleting old entries...")
			//		defer progressDialog.Window.Destroy()

			err := deleteOldEntries(config, targetOU, progressDialog)
			if err != nil {
				glib.IdleAdd(func() {
					showErrorDialog(win, "Failed to delete old entries: "+err.Error())
				})
				return
			}

			if progressDialog.IsCanceled() {
				return
			}

			progressDialog.SetLabel("Adding new entries...")
			err = addNewEntries(config, targetOU, entries, progressDialog)
			if err != nil {
				glib.IdleAdd(func() {
					showErrorDialog(win, "Failed to add new entries: "+err.Error())
				})
				return
			}

			glib.IdleAdd(func() {
				showInfoDialog(win, "Data loaded successfully!")
			})

		}()
	})

	btnBox.PackStart(buildTreeBtn, true, true, 0)
	btnBox.PackStart(loadBtn, true, true, 0)
	grid.Attach(btnBox, 0, 8, 3, 1)

	win.Add(grid)
	return win, nil
}

func getOUs(config LDAPConfig) ([]string, error) {
	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%s", config.Host, config.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP server: %v", err)
	}
	defer conn.Close()

	err = conn.Bind(config.BindDN, config.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to bind to LDAP server: %v", err)
	}

	searchRequest := ldap.NewSearchRequest(
		"ou=abook,"+config.BaseDN,
		ldap.ScopeSingleLevel, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=organizationalUnit)",
		[]string{"ou"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to search OUs: %v", err)
	}

	var ous []string
	for _, entry := range result.Entries {
		ous = append(ous, entry.GetAttributeValue("ou"))
	}

	return ous, nil
}

func parseLDIF(filename string) ([]LDIFEntry, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var entries []LDIFEntry
	var currentEntry LDIFEntry
	var inEntry bool

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			if inEntry {
				entries = append(entries, currentEntry)
				currentEntry = LDIFEntry{}
				inEntry = false
			}
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "dn":
			currentEntry.DN = value
			inEntry = true
		case "objectclass":
			currentEntry.ObjectClass = value
		case "sn":
			currentEntry.SN = value
		case "cn":
			currentEntry.CN = value
		case "ou":
			currentEntry.OU = value
		case "title":
			currentEntry.Title = value
		case "mail":
			currentEntry.Mail = value
		case "givenName":
			currentEntry.GivenName = value
		case "initials":
			currentEntry.Initials = value
		case "telephoneNumber":
			currentEntry.TelephoneNumber = value
		case "l":
			currentEntry.L = value
		case "postalAddress":
			currentEntry.PostalAddress = value
		case "o":
			currentEntry.O = value
		}
	}

	if inEntry {
		entries = append(entries, currentEntry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return entries, nil
}

// OrgNode represents a node in the organizational tree
type OrgNode struct {
	Name     string
	Children map[string]*OrgNode
}

func buildOrgTree(entries []LDIFEntry) *OrgNode {
	root := &OrgNode{
		Name:     "Organization",
		Children: make(map[string]*OrgNode),
	}

	for _, entry := range entries {
		str := entry.O
		if str == "filial" || len(str) == 0 {
			continue
		}

		orgParts := strings.SplitN(entry.O, ",", 2)
		orgName := strings.TrimSpace(orgParts[0])
		var deptName string
		if len(orgParts) > 1 {
			deptName = strings.TrimSpace(orgParts[1])
		}

		// Find or create organization node
		orgNode, exists := root.Children[orgName]
		if !exists {
			orgNode = &OrgNode{
				Name:     orgName,
				Children: make(map[string]*OrgNode),
			}
			root.Children[orgName] = orgNode
		}

		// Handle department and OU
		if deptName != "" {
			// Organization has departments
			deptNode, exists := orgNode.Children[deptName]
			if !exists {
				deptNode = &OrgNode{
					Name:     deptName,
					Children: make(map[string]*OrgNode),
				}
				orgNode.Children[deptName] = deptNode
			}

			// Add OU under department
			if entry.OU != "" {
				if _, exists := deptNode.Children[entry.OU]; !exists {
					deptNode.Children[entry.OU] = &OrgNode{
						Name:     entry.OU,
						Children: make(map[string]*OrgNode),
					}
				}
			}
		} else {
			// Organization has no departments, add OU directly under org
			if entry.OU != "" {
				if _, exists := orgNode.Children[entry.OU]; !exists {
					orgNode.Children[entry.OU] = &OrgNode{
						Name:     entry.OU,
						Children: make(map[string]*OrgNode),
					}
				}
			}
		}
	}

	return root
}

func showTreeWindow(parent *gtk.Window, root *OrgNode, entries []LDIFEntry, config LDAPConfig) {

	// Create tree window
	treeWindow, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Println("Error creating tree window:", err)
		return
	}

	treeWindow.SetTitle("Organizational Structure")
	treeWindow.SetDefaultSize(500, 600)
	treeWindow.SetTransientFor(parent)
	treeWindow.SetModal(true)

	// Create main container
	mainBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
	if err != nil {
		log.Println("Error creating main box:", err)
		treeWindow.Destroy()
		return
	}

	// Create button box
	buttonBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 5)
	if err != nil {
		log.Println("Error creating button box:", err)
		treeWindow.Destroy()
		return
	}

	// Create export button
	exportBtn, err := gtk.ButtonNewWithLabel("Export to LDIF")
	if err != nil {
		log.Println("Error creating export button:", err)
		treeWindow.Destroy()
		return
	}
	////////////////////////////////////////////////////////////////////
	//	exportBtn.Connect("clicked", func() {
	//		exportToLDIF(parent, entries, config.BaseDN)
	//	})

	// Create close button
	closeBtn, err := gtk.ButtonNewWithLabel("Close")
	if err != nil {
		log.Println("Error creating close button:", err)
		treeWindow.Destroy()
		return
	}
	closeBtn.Connect("clicked", func() {
		treeWindow.Destroy()
	})

	buttonBox.PackEnd(closeBtn, false, false, 0)
	buttonBox.PackEnd(exportBtn, false, false, 5)
	mainBox.PackStart(buttonBox, false, false, 5)

	// Create scrolled window
	scrolledWindow, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		log.Println("Error creating scrolled window:", err)
		treeWindow.Destroy()
		return
	}

	// Create tree view
	treeView, err := gtk.TreeViewNew()
	if err != nil {
		log.Println("Error creating tree view:", err)
		treeWindow.Destroy()
		return
	}

	// Create tree store
	treeStore, err := gtk.TreeStoreNew(glib.TYPE_STRING)
	if err != nil {
		log.Println("Error creating tree store:", err)
		treeWindow.Destroy()
		return
	}

	// Populate tree store
	populateTreeStore(treeStore, nil, root)

	// Create column
	column, err := gtk.TreeViewColumnNew()
	if err != nil {
		log.Println("Error creating tree column:", err)
		treeWindow.Destroy()
		return
	}

	column.SetTitle("Organizational Structure")

	exportBtn.Connect("clicked", func() {
		exportTreeToLDIF(parent, treeStore, root)
	})

	renderer, err := gtk.CellRendererTextNew()
	if err != nil {
		log.Println("Error creating cell renderer:", err)
		treeWindow.Destroy()
		return
	}

	column.PackStart(renderer, true)
	column.AddAttribute(renderer, "text", 0)
	treeView.AppendColumn(column)

	treeView.SetModel(treeStore)
	scrolledWindow.Add(treeView)
	mainBox.PackStart(scrolledWindow, true, true, 0)

	treeWindow.Add(mainBox)
	treeWindow.ShowAll()
}

// Helper function to populate tree store
func populateTreeStore(store *gtk.TreeStore, parent *gtk.TreeIter, node *OrgNode) {
	iter := store.Append(parent)
	store.SetValue(iter, 0, node.Name)

	for _, child := range node.Children {
		populateTreeStore(store, iter, child)
	}
}

func deleteOldEntries(config LDAPConfig, targetOU string, progress *ProgressDialog) error {
	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%s", config.Host, config.Port))
	if err != nil {
		return fmt.Errorf("failed to connect to LDAP server: %v", err)
	}
	defer conn.Close()

	err = conn.Bind(config.BindDN, config.Password)
	if err != nil {
		return fmt.Errorf("failed to bind to LDAP server: %v", err)
	}

	searchRequest := ldap.NewSearchRequest(
		"ou="+targetOU+",ou=abook,"+config.BaseDN,
		ldap.ScopeSingleLevel, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=inetOrgPerson)",
		[]string{"dn"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return fmt.Errorf("failed to search entries: %v", err)
	}

	total := len(result.Entries)
	for i, entry := range result.Entries {
		if progress.IsCanceled() {
			return fmt.Errorf("operation canceled by user")
		}

		delRequest := ldap.NewDelRequest(entry.DN, nil)
		if err := conn.Del(delRequest); err != nil {
			return fmt.Errorf("failed to delete entry %s: %v", entry.DN, err)
		}

		len := float64(i+1) / float64(total)

		glib.IdleAdd(func() {
			progress.Progress.SetFraction(len)
			progress.Label.SetText(fmt.Sprintf("Deleting entries... %d/%d", i+1, total))
		})
	}

	return nil
}

func addNewEntries(config LDAPConfig, targetOU string, entries []LDIFEntry, progress *ProgressDialog) error {
	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%s", config.Host, config.Port))
	if err != nil {
		return fmt.Errorf("failed to connect to LDAP server: %v", err)
	}
	defer conn.Close()

	err = conn.Bind(config.BindDN, config.Password)
	if err != nil {
		return fmt.Errorf("failed to bind to LDAP server: %v", err)
	}

	total := len(entries)
	for i, entry := range entries {
		if progress.IsCanceled() {
			return fmt.Errorf("operation canceled by user")
		}

		dn := fmt.Sprintf("cn=%s,ou=%s,ou=abook,%s", entry.CN, targetOU, config.BaseDN)
		addRequest := ldap.NewAddRequest(dn, nil)

		addRequest.Attribute("objectClass", []string{"inetOrgPerson"})
		addRequest.Attribute("sn", []string{entry.SN})
		addRequest.Attribute("cn", []string{entry.CN})
		addRequest.Attribute("ou", []string{entry.OU})
		if entry.Title != "" {
			addRequest.Attribute("title", []string{entry.Title})
		}
		if entry.Mail != "" {
			addRequest.Attribute("mail", []string{entry.Mail})
		}
		if entry.GivenName != "" {
			addRequest.Attribute("givenName", []string{entry.GivenName})
		}
		if entry.Initials != "" {
			addRequest.Attribute("initials", []string{entry.Initials})
		}
		if entry.TelephoneNumber != "" {
			addRequest.Attribute("telephoneNumber", []string{entry.TelephoneNumber})
		}
		if entry.L != "" {
			addRequest.Attribute("l", []string{entry.L})
		}
		if entry.PostalAddress != "" {
			addRequest.Attribute("postalAddress", []string{entry.PostalAddress})
		}
		if entry.O != "" {
			addRequest.Attribute("o", []string{entry.O})
		}

		if err := conn.Add(addRequest); err != nil {
			return fmt.Errorf("failed to add entry %s: %v", dn, err)
		}

		progressVal := float64(i+1) / float64(total)
		glib.IdleAdd(func() {
			progress.Progress.SetFraction(progressVal)
			progress.Label.SetText(fmt.Sprintf("Adding entries... %d/%d", i+1, total))
		})
	}

	return nil
}

func createProgressDialog(parent *gtk.Window, title, initialMessage string) *ProgressDialog {
	dialog, err := gtk.DialogNew()
	if err != nil {
		log.Println("Error creating progress dialog:", err)
		return nil
	}

	dialog.SetTitle(title)
	dialog.SetTransientFor(parent)
	dialog.SetModal(true)
	dialog.SetDefaultSize(300, 150)

	contentArea, err := dialog.GetContentArea()
	if err != nil {
		log.Println("Error getting content area:", err)
		dialog.Destroy()
		return nil
	}

	box, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 10)
	if err != nil {
		log.Println("Error creating box:", err)
		dialog.Destroy()
		return nil
	}
	box.SetBorderWidth(10)

	label, err := gtk.LabelNew(initialMessage)
	if err != nil {
		log.Println("Error creating label:", err)
		dialog.Destroy()
		return nil
	}

	progressBar, err := gtk.ProgressBarNew()
	if err != nil {
		log.Println("Error creating progress bar:", err)
		dialog.Destroy()
		return nil
	}

	cancelBtn, err := gtk.ButtonNewWithLabel("Cancel")
	if err != nil {
		log.Println("Error creating cancel button:", err)
		dialog.Destroy()
		return nil
	}

	box.PackStart(label, false, false, 0)
	box.PackStart(progressBar, false, false, 0)
	box.PackStart(cancelBtn, false, false, 0)
	contentArea.Add(box)

	pd := &ProgressDialog{
		Window:    dialog,
		Progress:  progressBar,
		Label:     label,
		CancelBtn: cancelBtn,
		Canceled:  false,
	}

	cancelBtn.Connect("clicked", func() {
		pd.Mutex.Lock()
		pd.Canceled = true
		pd.Mutex.Unlock()
	})

	dialog.ShowAll()

	return pd
}

func (pd *ProgressDialog) SetLabel(text string) {
	glib.IdleAdd(func() {
		pd.Label.SetText(text)
	})
}

func (pd *ProgressDialog) IsCanceled() bool {
	pd.Mutex.Lock()
	defer pd.Mutex.Unlock()
	return pd.Canceled
}

func showErrorDialog(parent *gtk.Window, message string) {
	dialog := gtk.MessageDialogNew(
		parent,
		gtk.DIALOG_MODAL,
		gtk.MESSAGE_ERROR,
		gtk.BUTTONS_OK,
		message,
	)
	dialog.Run()
	dialog.Destroy()
}

func showInfoDialog(parent *gtk.Window, message string) {
	dialog := gtk.MessageDialogNew(
		parent,
		gtk.DIALOG_MODAL,
		gtk.MESSAGE_INFO,
		gtk.BUTTONS_OK,
		message,
	)
	dialog.Run()
	dialog.Destroy()
}

func exportTreeToLDIF(parent *gtk.Window, treeStore *gtk.TreeStore, node *OrgNode) {
	// Create save file dialog
	saveDialog, err := gtk.FileChooserDialogNewWith2Buttons(
		"Save as LDIF",
		parent,
		gtk.FILE_CHOOSER_ACTION_SAVE,
		"Cancel",
		gtk.RESPONSE_CANCEL,
		"Save",
		gtk.RESPONSE_ACCEPT,
	)
	if err != nil {
		showErrorDialog(parent, "Error creating save dialog: "+err.Error())
		return
	}
	defer saveDialog.Destroy()

	// Set file filter
	filter, err := gtk.FileFilterNew()
	if err != nil {
		showErrorDialog(parent, "Error creating file filter: "+err.Error())
		return
	}
	filter.SetName("LDIF Files")
	filter.AddPattern("*.ldif")
	saveDialog.AddFilter(filter)
	saveDialog.SetCurrentName("structure.ldif")

	if saveDialog.Run() != gtk.RESPONSE_ACCEPT {
		return
	}

	// Get filename
	filename := saveDialog.GetFilename()
	if !strings.HasSuffix(filename, ".ldif") {
		filename += ".ldif"
	}

	// Create output file
	file, err := os.Create(filename)
	if err != nil {
		showErrorDialog(parent, "Error creating file: "+err.Error())
		return
	}
	defer file.Close()
	////////////////////////////////////////////////////////////////////////////////////////
	// Generate LDIF content

	/*
		iter := store.Append(parent)
		store.SetValue(iter, 0, node.Name)

		for _, child := range node.Children {
			populateTreeStore(store, iter, child)
		}

		node.Name
		node.Children

		var buf bytes.Buffer
		for _, entry := range entries {
			buf.WriteString(fmt.Sprintf("dn: cn=%s,%s\n", entry.DN, baseDN))
			buf.WriteString("objectClass: inetOrgPerson\n")
			buf.WriteString(fmt.Sprintf("cn: %s\n", entry.CN))
			buf.WriteString(fmt.Sprintf("sn: %s\n", entry.SN))

			if entry.OU != "" {
				buf.WriteString(fmt.Sprintf("ou: %s\n", entry.OU))
			}
			if entry.Title != "" {
				buf.WriteString(fmt.Sprintf("title: %s\n", entry.Title))
			}
			if entry.Mail != "" {
				buf.WriteString(fmt.Sprintf("mail: %s\n", entry.Mail))
			}
			if entry.GivenName != "" {
				buf.WriteString(fmt.Sprintf("givenName: %s\n", entry.GivenName))
			}
			if entry.Initials != "" {
				buf.WriteString(fmt.Sprintf("initials: %s\n", entry.Initials))
			}
			if entry.TelephoneNumber != "" {
				buf.WriteString(fmt.Sprintf("telephoneNumber: %s\n", entry.TelephoneNumber))
			}
			if entry.L != "" {
				buf.WriteString(fmt.Sprintf("l: %s\n", entry.L))
			}
			if entry.PostalAddress != "" {
				buf.WriteString(fmt.Sprintf("postalAddress: %s\n", entry.PostalAddress))
			}
			if entry.O != "" {
				buf.WriteString(fmt.Sprintf("o: %s\n", entry.O))
			}
			buf.WriteString("\n")
		}
	*/
	////////////////////////////////////////////////////////////////////////////////////////

	// Write to file
	if _, err := file.Write(buf.Bytes()); err != nil {
		showErrorDialog(parent, "Error writing to file: "+err.Error())
		return
	}

	// showInfoDialog(parent, fmt.Sprintf(
	//
	//	"Successfully exported %d entries to:\n%s",
	//	len(entries),
	//	filename,
	//
	// ))
}

func exportToLDIF(parent *gtk.Window, entries []LDIFEntry, baseDN string) {
	// Create save file dialog
	saveDialog, err := gtk.FileChooserDialogNewWith2Buttons(
		"Save as LDIF",
		parent,
		gtk.FILE_CHOOSER_ACTION_SAVE,
		"Cancel",
		gtk.RESPONSE_CANCEL,
		"Save",
		gtk.RESPONSE_ACCEPT,
	)
	if err != nil {
		showErrorDialog(parent, "Error creating save dialog: "+err.Error())
		return
	}
	defer saveDialog.Destroy()

	// Set file filter
	filter, err := gtk.FileFilterNew()
	if err != nil {
		showErrorDialog(parent, "Error creating file filter: "+err.Error())
		return
	}
	filter.SetName("LDIF Files")
	filter.AddPattern("*.ldif")
	saveDialog.AddFilter(filter)
	saveDialog.SetCurrentName("structure.ldif")

	if saveDialog.Run() != gtk.RESPONSE_ACCEPT {
		return
	}

	// Get filename
	filename := saveDialog.GetFilename()
	if !strings.HasSuffix(filename, ".ldif") {
		filename += ".ldif"
	}

	// Create output file
	file, err := os.Create(filename)
	if err != nil {
		showErrorDialog(parent, "Error creating file: "+err.Error())
		return
	}
	defer file.Close()

	// Generate LDIF content
	var buf bytes.Buffer
	for _, entry := range entries {
		buf.WriteString(fmt.Sprintf("dn: cn=%s,%s\n", entry.DN, baseDN))
		buf.WriteString("objectClass: inetOrgPerson\n")
		buf.WriteString(fmt.Sprintf("cn: %s\n", entry.CN))
		buf.WriteString(fmt.Sprintf("sn: %s\n", entry.SN))

		if entry.OU != "" {
			buf.WriteString(fmt.Sprintf("ou: %s\n", entry.OU))
		}
		if entry.Title != "" {
			buf.WriteString(fmt.Sprintf("title: %s\n", entry.Title))
		}
		if entry.Mail != "" {
			buf.WriteString(fmt.Sprintf("mail: %s\n", entry.Mail))
		}
		if entry.GivenName != "" {
			buf.WriteString(fmt.Sprintf("givenName: %s\n", entry.GivenName))
		}
		if entry.Initials != "" {
			buf.WriteString(fmt.Sprintf("initials: %s\n", entry.Initials))
		}
		if entry.TelephoneNumber != "" {
			buf.WriteString(fmt.Sprintf("telephoneNumber: %s\n", entry.TelephoneNumber))
		}
		if entry.L != "" {
			buf.WriteString(fmt.Sprintf("l: %s\n", entry.L))
		}
		if entry.PostalAddress != "" {
			buf.WriteString(fmt.Sprintf("postalAddress: %s\n", entry.PostalAddress))
		}
		if entry.O != "" {
			buf.WriteString(fmt.Sprintf("o: %s\n", entry.O))
		}
		buf.WriteString("\n")
	}

	// Write to file
	if _, err := file.Write(buf.Bytes()); err != nil {
		showErrorDialog(parent, "Error writing to file: "+err.Error())
		return
	}

	showInfoDialog(parent, fmt.Sprintf(
		"Successfully exported %d entries to:\n%s",
		len(entries),
		filename,
	))
}
