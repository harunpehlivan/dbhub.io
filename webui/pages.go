package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/icza/session"
	"github.com/rhinoman/go-commonmark"
	com "github.com/sqlitebrowser/dbhub.io/common"
)

// Renders the "About Us" page.
func aboutPage(w http.ResponseWriter, r *http.Request) {
	var pageData struct {
		Auth0 com.Auth0Set
		Meta  com.MetaInfo
	}

	// Retrieve session data (if any)
	var loggedInUser string
	sess := session.Get(r)
	if sess != nil {
		u := sess.CAttr("UserName")
		if u != nil {
			loggedInUser = u.(string)
			pageData.Meta.LoggedInUser = loggedInUser
		} else {
			session.Remove(sess, w)
		}
	}

	pageData.Meta.Title = "What is DBHub.io?"

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("aboutPage")
	err := t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

// Render the branches page, which lists the branches for a database.
func branchesPage(w http.ResponseWriter, r *http.Request) {
	// Structure to hold page data
	type brEntry struct {
		Commit      string `json:"commit"`
		Description string `json:"description"`
		Name        string `json:"name"`
	}
	var pageData struct {
		Auth0         com.Auth0Set
		Branches      []brEntry
		DB            com.SQLiteDBinfo
		DefaultBranch string
		Meta          com.MetaInfo
	}
	pageData.Meta.Title = "Branch list"

	// Retrieve session data (if any)
	var loggedInUser string
	sess := session.Get(r)
	if sess != nil {
		u := sess.CAttr("UserName")
		if u != nil {
			loggedInUser = u.(string)
			pageData.Meta.LoggedInUser = loggedInUser
		} else {
			session.Remove(sess, w)
		}
	}

	// Retrieve the database owner & name, and branch name
	// TODO: Add folder support
	dbFolder := "/"
	dbOwner, dbName, err := com.GetOD(1, r) // 1 = Ignore "/branches/" at the start of the URL
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// Validate the supplied information
	if dbOwner == "" || dbName == "" {
		errorPage(w, r, http.StatusBadRequest, "Missing database owner or database name")
		return
	}

	// Check if the requested database exists
	exists, err := com.CheckDBExists(dbOwner, dbFolder, dbName)
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if !exists {
		errorPage(w, r, http.StatusBadRequest, fmt.Sprintf("Database '%s%s%s' doesn't exist", dbOwner, dbFolder,
			dbName))
		return
	}

	// Check if the user has access to the requested database (and get it's details if available)
	err = com.DBDetails(&pageData.DB, loggedInUser, dbOwner, "/", dbName, "")
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// Read the branch heads list from the database
	branches, err := com.GetBranches(dbOwner, dbFolder, dbName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	pageData.DefaultBranch, err = com.GetDefaultBranchName(dbOwner, dbFolder, dbName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Fill out the metadata
	pageData.Meta.Owner = dbOwner
	pageData.Meta.Database = dbName
	for i, j := range branches {
		k := brEntry{
			Commit:      j.Commit,
			Description: j.Description,
			Name:        i,
		}
		pageData.Branches = append(pageData.Branches, k)
	}

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("branchesPage")
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

// Render the commits page.  This shows all of the commits in a given branch, in reverse order from newest to oldest.
func commitsPage(w http.ResponseWriter, r *http.Request) {
	// Structure to hold page data
	type HistEntry struct {
		AuthorEmail    string     `json:"author_email"`
		AuthorName     string     `json:"author_name"`
		AuthorUserName string     `json:"author_user_name"`
		CommitterEmail string     `json:"committer_email"`
		CommitterName  string     `json:"committer_name"`
		ID             string     `json:"id"`
		Message        string     `json:"message"`
		Parent         string     `json:"parent"`
		Timestamp      time.Time  `json:"timestamp"`
		Tree           com.DBTree `json:"tree"`
	}
	var pageData struct {
		Auth0    com.Auth0Set
		Branch   string
		Branches []string
		DB       com.SQLiteDBinfo
		History  []HistEntry
		Meta     com.MetaInfo
	}
	pageData.Meta.Title = "Commits settings"

	// Retrieve session data (if any)
	var loggedInUser string
	sess := session.Get(r)
	if sess != nil {
		u := sess.CAttr("UserName")
		if u != nil {
			loggedInUser = u.(string)
			pageData.Meta.LoggedInUser = loggedInUser
		} else {
			session.Remove(sess, w)
		}
	}

	// Retrieve the database owner & name, and branch name
	// TODO: Add folder support
	dbFolder := "/"
	dbOwner, dbName, err := com.GetOD(1, r) // 1 = Ignore "/settings/" at the start of the URL
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}
	branchName, err := com.GetFormBranch(r)
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// Validate the supplied information
	if dbOwner == "" || dbName == "" {
		errorPage(w, r, http.StatusBadRequest, "Missing database owner or database name")
		return
	}

	// Check if the requested database exists
	exists, err := com.CheckDBExists(dbOwner, dbFolder, dbName)
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if !exists {
		errorPage(w, r, http.StatusBadRequest, fmt.Sprintf("Database '%s%s%s' doesn't exist", dbOwner, dbFolder,
			dbName))
		return
	}

	// Check if the user has access to the requested database (and get it's details if available)
	err = com.DBDetails(&pageData.DB, loggedInUser, dbOwner, "/", dbName, "")
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// Read the branch heads list from the database
	branches, err := com.GetBranches(dbOwner, dbFolder, dbName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// If no branch name was given, we use the default branch
	if branchName == "" {
		branchName, err = com.GetDefaultBranchName(dbOwner, dbFolder, dbName)
		if err != nil {
			errorPage(w, r, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Work out the head commit ID for the requested branch
	headCom, ok := branches[branchName]
	headID := headCom.Commit
	if !ok {
		// Unknown branch
		errorPage(w, r, http.StatusInternalServerError, fmt.Sprintf("Branch '%s' not found", branchName))
		return
	}
	if headID == "" {
		// The requested branch wasn't found.  Bad request?
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// Walk the commit history backwards from the head commit, assembling the commit history for this branch from the
	// full list
	rawList, err := com.GetCommitList(dbOwner, dbFolder, dbName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	// TODO: Ugh, this is an ugly approach just to add the username to the commit data.  Surely there's a better way?
	// TODO  Maybe store the username in the commit data structure in the database instead?
	// TODO: Display licence changes too
	eml, err := com.GetUsernameFromEmail(rawList[headID].AuthorEmail)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	pageData.History = []HistEntry{
		{
			AuthorEmail:    rawList[headID].AuthorEmail,
			AuthorName:     rawList[headID].AuthorName,
			AuthorUserName: eml,
			CommitterEmail: rawList[headID].CommitterEmail,
			CommitterName:  rawList[headID].CommitterName,
			ID:             rawList[headID].ID,
			Message:        commonmark.Md2Html(rawList[headID].Message, commonmark.CMARK_OPT_DEFAULT),
			Parent:         rawList[headID].Parent,
			Timestamp:      rawList[headID].Timestamp,
			Tree:           rawList[headID].Tree,
		},
	}
	commitData := com.CommitEntry{Parent: rawList[headID].Parent}
	for commitData.Parent != "" {
		commitData, ok = rawList[commitData.Parent]
		if !ok {
			errorPage(w, r, http.StatusInternalServerError, "Internal error when retrieving commit data")
			return
		}
		eml, err := com.GetUsernameFromEmail(commitData.AuthorEmail)
		if err != nil {
			errorPage(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		newEntry := HistEntry{
			AuthorEmail:    commitData.AuthorEmail,
			AuthorName:     commitData.AuthorName,
			AuthorUserName: eml,
			CommitterEmail: commitData.CommitterEmail,
			CommitterName:  commitData.CommitterName,
			ID:             commitData.ID,
			Message:        commonmark.Md2Html(commitData.Message, commonmark.CMARK_OPT_DEFAULT),
			Parent:         commitData.Parent,
			Timestamp:      commitData.Timestamp,
			Tree:           commitData.Tree,
		}
		pageData.History = append(pageData.History, newEntry)
	}

	// Fill out the metadata
	pageData.Meta.Owner = dbOwner
	pageData.Meta.Database = dbName
	pageData.Branch = branchName
	for i := range branches {
		pageData.Branches = append(pageData.Branches, i)
	}

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("commitsPage")
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

// Displays a web page asking for the new branch name.
func createBranchPage(w http.ResponseWriter, r *http.Request) {
	var pageData struct {
		Auth0  com.Auth0Set
		Meta   com.MetaInfo
		Commit string
	}
	pageData.Meta.Title = "Create new branch"

	// Retrieve session data (if any)
	var loggedInUser string
	validSession := false
	sess := session.Get(r)
	if sess != nil {
		u := sess.CAttr("UserName")
		if u != nil {
			loggedInUser = u.(string)
			pageData.Meta.LoggedInUser = loggedInUser
			validSession = true
		} else {
			session.Remove(sess, w)
		}
	}
	if validSession != true {
		// Display an error message
		errorPage(w, r, http.StatusForbidden, "Error: Must be logged in to view that page.")
		return
	}

	// Retrieve the owner, database, and commit ID
	var err error
	dbOwner, dbName, commit, err := com.GetODC(1, r) // "1" means skip the first URL word
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, "Validation failed for commit value")
		return
	}
	// TODO: Add folder support
	dbFolder := "/"

	// Check if the requested database exists
	exists, err := com.CheckDBExists(dbOwner, dbFolder, dbName)
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if !exists {
		errorPage(w, r, http.StatusBadRequest, fmt.Sprintf("Database '%s%s%s' doesn't exist", dbOwner, dbFolder,
			dbName))
		return
	}

	// Make sure the database owner matches the logged in user
	if loggedInUser != dbOwner {
		errorPage(w, r, http.StatusUnauthorized, "You can't change databases you don't own")
		return
	}

	// Fill out metadata for the page to be rendered
	pageData.Meta.Owner = dbOwner
	pageData.Meta.Database = dbName
	pageData.Commit = commit

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("createBranchPage")
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

func databasePage(w http.ResponseWriter, r *http.Request, dbOwner string, dbName string, commitID string, dbTable string, sortCol string, sortDir string, rowOffset int) {
	pageName := "Render database page"

	var pageData struct {
		Auth0  com.Auth0Set
		Data   com.SQLiteRecordSet
		DB     com.SQLiteDBinfo
		Meta   com.MetaInfo
		MyStar bool
	}

	// Retrieve session data (if any)
	var loggedInUser string
	sess := session.Get(r)
	if sess != nil {
		u := sess.CAttr("UserName")
		if u != nil {
			loggedInUser = u.(string)
			pageData.Meta.LoggedInUser = loggedInUser
		} else {
			session.Remove(sess, w)
		}
	}

	// Check if the user has access to the requested database (and get it's details if available)
	// TODO: Add proper folder support
	err := com.DBDetails(&pageData.DB, loggedInUser, dbOwner, "/", dbName, commitID)
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// * Execution can only get here if the user has access to the requested database *

	// Check if the database was starred by the logged in user
	myStar, err := com.CheckDBStarred(loggedInUser, dbOwner, "/", dbName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, "Couldn't retrieve latest social stats")
		return
	}

	// If a specific table wasn't requested, use the user specified default (if present)
	if dbTable == "" {
		dbTable = pageData.DB.Info.DefaultTable
	}

	// Determine the number of rows to display
	var tempMaxRows int
	if loggedInUser != "" {
		tempMaxRows = com.PrefUserMaxRows(loggedInUser)
		pageData.DB.MaxRows = tempMaxRows
	} else {
		// Not logged in, so use the default number of rows
		tempMaxRows = com.DefaultNumDisplayRows
		pageData.DB.MaxRows = tempMaxRows
	}

	// Generate predictable cache keys for the metadata and sqlite table rows
	mdataCacheKey := com.MetadataCacheKey("dwndb-meta", loggedInUser, dbOwner, "/", dbName,
		commitID)
	rowCacheKey := com.TableRowsCacheKey(fmt.Sprintf("tablejson/%s/%s/%d", sortCol, sortDir, rowOffset),
		loggedInUser, dbOwner, "/", dbName, commitID, dbTable, pageData.DB.MaxRows)

	// If a cached version of the page data exists, use it
	ok, err := com.GetCachedData(mdataCacheKey, &pageData)
	if err != nil {
		log.Printf("%s: Error retrieving page data from cache: %v\n", pageName, err)
	}
	if ok {
		// Grab the cached table data as well
		ok, err := com.GetCachedData(rowCacheKey, &pageData.Data)
		if err != nil {
			log.Printf("%s: Error retrieving page data from cache: %v\n", pageName, err)
		}

		// Restore the correct MaxRow value
		pageData.DB.MaxRows = tempMaxRows

		// Restore the correct username
		pageData.Meta.LoggedInUser = loggedInUser

		// Get latest star and fork count
		_, pageData.DB.Info.Stars, pageData.DB.Info.Forks, err = com.SocialStats(dbOwner, "/", dbName)
		if err != nil {
			errorPage(w, r, http.StatusInternalServerError, err.Error())
			return
		}

		// Render the page (using the caches)
		if ok {
			t := tmpl.Lookup("databasePage")
			err = t.Execute(w, pageData)
			if err != nil {
				log.Printf("Error: %s", err)
			}
			return
		}

		// Note - If the row data wasn't found in cache, we fall through and continue on with the rest of this
		//        function, which grabs it and caches it for future use
	}

	// Get a handle from Minio for the database object
	sdb, tempFile, err := com.OpenMinioObject(pageData.DB.Info.DBEntry.Sha256[:com.MinioFolderChars],
		pageData.DB.Info.DBEntry.Sha256[com.MinioFolderChars:])
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Close the SQLite database and delete the temp file
	defer func() {
		sdb.Close()
		os.Remove(tempFile)
	}()

	// Retrieve the list of tables in the database
	tables, err := com.Tables(sdb, dbName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	pageData.DB.Info.Tables = tables

	// If a specific table was requested, check that it's present
	if dbTable != "" {
		// Check the requested table is present
		tablePresent := false
		for _, tbl := range tables {
			if tbl == dbTable {
				tablePresent = true
			}
		}
		if tablePresent == false {
			// The requested table doesn't exist in the database
			log.Printf("%s: Requested table not present in database. DB: '%s/%s', Table: '%s'\n",
				pageName, dbOwner, dbName, dbTable)
			errorPage(w, r, http.StatusBadRequest, "Requested table not present")
			return
		}
	}

	// If a specific table wasn't requested, use the first table in the database
	if dbTable == "" {
		dbTable = pageData.DB.Info.Tables[0]
	}

	// If a sort column was requested, verify it exists
	if sortCol != "" {
		colList, err := sdb.Columns("", dbTable)
		if err != nil {
			log.Printf("Error when reading column names for table '%s': %v\n", dbTable,
				err.Error())
			errorPage(w, r, http.StatusInternalServerError, "Error when reading from the database")
			return
		}
		colExists := false
		for _, j := range colList {
			if j.Name == sortCol {
				colExists = true
			}
		}
		if colExists == false {
			// The requested sort column doesn't exist, so we fall back to no sorting
			sortCol = ""
		}
	}

	// Validate the table name, just to be careful
	err = com.ValidatePGTable(dbTable)
	if err != nil {
		// Validation failed, so don't pass on the table name

		// If the failed table name is "{{ db.Tablename }}", don't bother logging it.  It's just a search
		// bot picking up AngularJS in a string and doing a request with it
		if dbTable != "{{ db.Tablename }}" {
			log.Printf("%s: Validation failed for table name: '%s': %s", pageName, dbTable, err)
		}
		errorPage(w, r, http.StatusBadRequest, "Validation failed for table name")
		return
	}

	// Fill out various metadata fields
	pageData.Meta.Owner = dbOwner
	pageData.Meta.Database = dbName
	pageData.Meta.Server = com.WebServer()
	pageData.Meta.Title = fmt.Sprintf("%s / %s", dbOwner, dbName)

	// Retrieve the "forked from" information
	frkOwn, frkFol, frkDB, err := com.ForkedFrom(dbOwner, "/", dbName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, "Database query failure")
		return
	}
	pageData.Meta.ForkOwner = frkOwn
	pageData.Meta.ForkFolder = frkFol
	pageData.Meta.ForkDatabase = frkDB

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Update database star status for the logged in user
	pageData.MyStar = myStar

	// Render the full description as markdown / CommonMark
	pageData.DB.Info.FullDesc = commonmark.Md2Html(pageData.DB.Info.FullDesc, commonmark.CMARK_OPT_DEFAULT)

	// Cache the page metadata
	err = com.CacheData(mdataCacheKey, pageData, com.CacheTime)
	if err != nil {
		log.Printf("%s: Error when caching page data: %v\n", pageName, err)
	}

	// Grab the cached table data if it's available
	ok, err = com.GetCachedData(rowCacheKey, &pageData.Data)
	if err != nil {
		log.Printf("%s: Error retrieving page data from cache: %v\n", pageName, err)
	}

	// If the row data wasn't in cache, read it from the database
	if !ok {
		pageData.Data, err = com.ReadSQLiteDB(sdb, dbTable, pageData.DB.MaxRows, sortCol, sortDir, rowOffset)
		if err != nil {
			// Some kind of error when reading the database data
			errorPage(w, r, http.StatusBadRequest, err.Error())
			return
		}
		pageData.Data.Tablename = dbTable
	}

	// Cache the table row data
	err = com.CacheData(rowCacheKey, pageData.Data, com.CacheTime)
	if err != nil {
		log.Printf("%s: Error when caching page data: %v\n", pageName, err)
	}

	// Render the page
	t := tmpl.Lookup("databasePage")
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

// General error display page.
func errorPage(w http.ResponseWriter, r *http.Request, httpcode int, msg string) {
	var pageData struct {
		Auth0   com.Auth0Set
		Message string
		Meta    com.MetaInfo
	}
	pageData.Message = msg

	// Retrieve session data (if any)
	var loggedInUser string
	sess := session.Get(r)
	if sess != nil {
		u := sess.CAttr("UserName")
		if u != nil {
			loggedInUser = u.(string)
			pageData.Meta.LoggedInUser = loggedInUser
		} else {
			session.Remove(sess, w)
		}
	}

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	w.WriteHeader(httpcode)
	t := tmpl.Lookup("errorPage")
	err := t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

// Render the page showing forks of the given database
func forksPage(w http.ResponseWriter, r *http.Request, dbOwner string, dbFolder string, dbName string) {
	var pageData struct {
		Auth0 com.Auth0Set
		Meta  com.MetaInfo
		Forks []com.ForkEntry
	}
	pageData.Meta.Title = "Forks"
	pageData.Meta.Owner = dbOwner
	pageData.Meta.Database = dbName

	// Retrieve session data (if any)
	var loggedInUser string
	sess := session.Get(r)
	if sess != nil {
		u := sess.CAttr("UserName")
		if u != nil {
			loggedInUser = u.(string)
			pageData.Meta.LoggedInUser = loggedInUser
		} else {
			session.Remove(sess, w)
		}
	}

	// Retrieve list of forks for the database
	var err error
	pageData.Forks, err = com.ForkTree(loggedInUser, dbOwner, dbFolder, dbName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError,
			fmt.Sprintf("Error retrieving fork list for '%s%s%s': %v\n", dbOwner, dbFolder,
				dbName, err.Error()))
		return
	}

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("forksPage")
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

// Renders the front page of the website.
func frontPage(w http.ResponseWriter, r *http.Request) {
	// Structure to hold page data
	var pageData struct {
		Auth0 com.Auth0Set
		List  []com.UserInfo
		Meta  com.MetaInfo
	}

	// Retrieve session data (if any)
	var loggedInUser string
	sess := session.Get(r)
	if sess != nil {
		u := sess.CAttr("UserName")
		if u != nil {
			loggedInUser = u.(string)
			pageData.Meta.LoggedInUser = loggedInUser
		} else {
			session.Remove(sess, w)
		}
	}

	// Retrieve list of users with public databases
	var err error
	pageData.List, err = com.PublicUserDBs()
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, "Database query failed")
		return
	}
	pageData.Meta.Title = `SQLite storage "in the cloud"`

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("rootPage")
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

// Renders the user Preferences page.
func prefPage(w http.ResponseWriter, r *http.Request, loggedInUser string) {
	var pageData struct {
		Auth0       com.Auth0Set
		DisplayName string
		Email       string
		MaxRows     int
		Meta        com.MetaInfo
	}
	pageData.Meta.Title = "Preferences"
	pageData.Meta.LoggedInUser = loggedInUser

	// Grab the user's display name and email address
	var err error
	pageData.DisplayName, pageData.Email, err = com.GetUserDetails(loggedInUser)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, "Database query failed")
		return
	}

	// Set the server name, used for the placeholder email address suggestion
	serverName := strings.Split(com.WebServer(), ":")
	pageData.Meta.Server = serverName[0]

	// Retrieve the user preference data
	pageData.MaxRows = com.PrefUserMaxRows(loggedInUser)

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("prefPage")
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

func profilePage(w http.ResponseWriter, r *http.Request, userName string) {
	var pageData struct {
		Auth0      com.Auth0Set
		Meta       com.MetaInfo
		PrivateDBs []com.DBInfo
		PublicDBs  []com.DBInfo
		Stars      []com.DBEntry
	}
	pageData.Meta.Owner = userName
	pageData.Meta.Title = userName
	pageData.Meta.Server = com.WebServer()
	pageData.Meta.LoggedInUser = userName

	// Check if the desired user exists
	userExists, err := com.CheckUserExists(userName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, "Database query failed")
		return
	}

	// If the user doesn't exist, indicate that
	if !userExists {
		errorPage(w, r, http.StatusNotFound, fmt.Sprintf("Unknown user: %s", userName))
		return
	}

	// Retrieve list of public databases for the user
	pageData.PublicDBs, err = com.UserDBs(userName, com.DB_PUBLIC)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, "Database query failed")
		return
	}

	// Retrieve list of private databases for the user
	pageData.PrivateDBs, err = com.UserDBs(userName, com.DB_PRIVATE)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, "Database query failed")
		return
	}

	// Retrieve the list of starred databases for the user
	pageData.Stars, err = com.UserStarredDBs(userName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, "Database query failed")
		return
	}

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("profilePage")
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

// Displays a web page for new users to choose their username.
func selectUserNamePage(w http.ResponseWriter, r *http.Request) {
	var pageData struct {
		Auth0 com.Auth0Set
		Meta  com.MetaInfo
		Nick  string
	}
	pageData.Meta.Title = "Select your username"

	// Retrieve session data (if any)
	sess := session.Get(r)
	if sess != nil {
		validRegSession := false
		va := sess.CAttr("registrationinprogress")
		if va == nil {
			// This isn't a valid username selection session, so abort
			errorPage(w, r, http.StatusBadRequest, "Invalid user creation session")
			return
		}
		validRegSession = va.(bool)

		if validRegSession != true {
			// For some reason this isn't a valid username selection session, so abort
			errorPage(w, r, http.StatusBadRequest, "Invalid user creation session")
			return
		}
	} else {
		errorPage(w, r, http.StatusBadRequest, "Invalid user creation session")
		return
	}

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// If the Auth0 profile included a nickname, we use that to pre-fill the input field
	ni := sess.CAttr("nickname")
	if ni != nil {
		pageData.Nick = ni.(string)
	}

	// Render the page
	t := tmpl.Lookup("selectUserNamePage")
	err := t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

// Render the settings page.
func settingsPage(w http.ResponseWriter, r *http.Request) {
	// Structure to hold page data
	var pageData struct {
		Auth0 com.Auth0Set
		DB    com.SQLiteDBinfo
		Meta  com.MetaInfo
	}
	pageData.Meta.Title = "Database settings"

	// Retrieve session data (if any)
	var loggedInUser string
	validSession := false
	sess := session.Get(r)
	if sess != nil {
		u := sess.CAttr("UserName")
		if u != nil {
			loggedInUser = u.(string)
			pageData.Meta.LoggedInUser = loggedInUser
			validSession = true
		} else {
			session.Remove(sess, w)
		}
	}
	if validSession != true {
		// Display an error message
		// TODO: Show the login dialog (also for the preferences page)
		errorPage(w, r, http.StatusForbidden, "Error: Must be logged in to view that page.")
		return
	}

	// Retrieve the database owner, database name
	// TODO: Add folder support
	dbOwner, dbName, err := com.GetOD(1, r) // 1 = Ignore "/settings/" at the start of the URL
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// Validate the supplied information
	if dbOwner == "" || dbName == "" {
		errorPage(w, r, http.StatusBadRequest, "Missing database owner or database name")
		return
	}
	if dbOwner != loggedInUser {
		errorPage(w, r, http.StatusBadRequest,
			"You can only access the settings page for your own databases")
		return
	}

	// Check if the user has access to the requested database (and get it's details if available)
	err = com.DBDetails(&pageData.DB, loggedInUser, dbOwner, "/", dbName, "")
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// Get a handle from Minio for the database object
	bkt := pageData.DB.Info.DBEntry.Sha256[:com.MinioFolderChars]
	id := pageData.DB.Info.DBEntry.Sha256[com.MinioFolderChars:]
	sdb, tempFile, err := com.OpenMinioObject(bkt, id)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Close the SQLite database and delete the temp file
	defer func() {
		sdb.Close()
		os.Remove(tempFile)
	}()

	// Retrieve the list of tables in the database
	pageData.DB.Info.Tables, err = com.Tables(sdb, fmt.Sprintf("%s%s%s", dbOwner, "/", dbName))
	defer sdb.Close()
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Fill out the metadata
	pageData.Meta.Owner = dbOwner
	pageData.Meta.Database = dbName

	// If the default table is blank, use the first one from the table list
	if pageData.DB.Info.DefaultTable == "" {
		pageData.DB.Info.DefaultTable = pageData.DB.Info.Tables[0]
	}

	// TODO: Hook up the real license choices
	pageData.DB.Info.License = com.OTHER

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("settingsPage")
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

// Present the stars page to the user.
func starsPage(w http.ResponseWriter, r *http.Request) {
	var pageData struct {
		Auth0 com.Auth0Set
		Meta  com.MetaInfo
		Stars []com.DBEntry
	}
	pageData.Meta.Title = "Stars"

	// Retrieve session data (if any)
	var loggedInUser string
	sess := session.Get(r)
	if sess != nil {
		u := sess.CAttr("UserName")
		if u != nil {
			loggedInUser = u.(string)
			pageData.Meta.LoggedInUser = loggedInUser
		} else {
			session.Remove(sess, w)
		}
	}

	// Retrieve owner and database name
	dbOwner, dbName, err := com.GetOD(1, r) // 1 = Ignore "/stars/" at the start of the URL
	if err != nil {
		errorPage(w, r, http.StatusBadRequest, err.Error())
		return
	}
	pageData.Meta.Owner = dbOwner
	pageData.Meta.Database = dbName

	// Retrieve list of users who starred the database
	pageData.Stars, err = com.UsersStarredDB(dbOwner, "/", dbName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, "Database query failed")
		return
	}

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("starsPage")
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

func uploadPage(w http.ResponseWriter, r *http.Request, userName string) {
	var pageData struct {
		Auth0 com.Auth0Set
		Meta  com.MetaInfo
	}
	pageData.Meta.Title = "Upload database"
	pageData.Meta.LoggedInUser = userName

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("uploadPage")
	err := t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}

func userPage(w http.ResponseWriter, r *http.Request, userName string) {
	// Structure to hold page data
	var pageData struct {
		Auth0  com.Auth0Set
		DBRows []com.DBInfo
		Meta   com.MetaInfo
	}
	pageData.Meta.Owner = userName
	pageData.Meta.Title = userName
	pageData.Meta.Server = com.WebServer()

	// Retrieve session data (if any)
	var loggedInUser string
	sess := session.Get(r)
	if sess != nil {
		u := sess.CAttr("UserName")
		if u != nil {
			loggedInUser = u.(string)
			if loggedInUser == userName {
				// The logged in user is looking at their own user page
				profilePage(w, r, loggedInUser)
				return
			}
			pageData.Meta.LoggedInUser = loggedInUser
		} else {
			session.Remove(sess, w)
		}
	}

	// Check if the desired user exists
	userExists, err := com.CheckUserExists(userName)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, "Database query failed")
		return
	}

	// If the user doesn't exist, indicate that
	if !userExists {
		errorPage(w, r, http.StatusNotFound, fmt.Sprintf("Unknown user: %s", userName))
		return
	}

	// Retrieve list of public databases for the user
	pageData.DBRows, err = com.UserDBs(userName, com.DB_PUBLIC)
	if err != nil {
		errorPage(w, r, http.StatusInternalServerError, "Database query failed")
		return
	}

	// Add Auth0 info to the page data
	pageData.Auth0.CallbackURL = "https://" + com.WebServer() + "/x/callback"
	pageData.Auth0.ClientID = com.Auth0ClientID()
	pageData.Auth0.Domain = com.Auth0Domain()

	// Render the page
	t := tmpl.Lookup("userPage")
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("Error: %s", err)
	}
}
