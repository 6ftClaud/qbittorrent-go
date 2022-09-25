/*
https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)
An attempt to implement the full qBittorrent Web API in Golang
*/

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/publicsuffix"
)

func init() {
	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.DebugLevel)
}

/*
Set the Client struct
*/
type Client struct {
	http          *http.Client
	URL           string
	Authenticated bool
}

/*
NewClient creates a new client connection to qBittorrent
*/
func NewClient(host string) *Client {
	// ensure url ends with a slash
	if host[len(host)-1:] != "/" {
		host += "/"
	}

	// Add the API url
	host += "api/v2/"
	client := &http.Client{}

	return &Client{
		http:          client,
		URL:           host,
		Authenticated: false,
	}
}

/*
Perform a GET request

	endpoint	string	Set the endpoint path, i.e. torrents/info
	opts	map[string]string	optional parameters (?username=usr&password=pswrd)
*/
func (client *Client) get(endpoint string, opts map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("GET", client.URL+endpoint, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build the request")
	}

	req.Header.Set("Referer", client.URL)

	// add optional parameters
	if opts != nil {
		query := req.URL.Query()
		for k, v := range opts {
			query.Add(k, v)
		}
		req.URL.RawQuery = query.Encode()
	}

	resp, err := client.http.Do(req)
	log.Debug("Sending GET request to ", req.URL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to execute the request")
	}
	return resp, nil
}

/*
Perform a POST request

	endpoint	string	Set the endpoint, i.e. app/shutdown
	opts map[string]string	optional post data
*/
func (client *Client) post(endpoint string, opts map[string]string) (*http.Response, error) {
	// add optional parameters
	params := url.Values{}
	for k, v := range opts {
		params.Add(k, v)
	}
	req, err := http.NewRequest("POST", client.URL+endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to build the request")
	}

	req.Header.Add("Referer", client.URL)
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	log.Debug("Sending POST request to ", req.URL, " with args ", params)
	resp, err := client.http.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to execute the request")
	}
	return resp, nil
}

func (Client) processList(key string, list []string) (hashMap map[string]string) {
	params := map[string]string{}
	entries := ""
	for i, v := range list {
		if i > 0 {
			entries += "|" + v
		} else {
			entries = v
		}
	}
	params[key] = entries
	return params
}

// Log in to qBittorrent, obtain a cookie for later use and set auth status to True
func (client *Client) Login(username string, password string) (bool, error) {
	credentials := make(map[string]string)
	credentials["username"] = username
	credentials["password"] = password

	resp, err := client.post("auth/login", credentials)
	if err != nil {
		return false, err
	} else if resp.Status != "200 OK" {
		return false, errors.Wrap(err, "User's IP is banned for too many failed login attempts")
	} else if len(resp.Cookies()) < 1 {
		return false, errors.Wrap(err, "No cookies in login response")
	}

	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	cookieURL, _ := url.Parse(client.URL)

	jar.SetCookies(cookieURL, []*http.Cookie{resp.Cookies()[0]})
	client.http.Jar = jar

	// change authentication status so we know were authenticated in later requests
	client.Authenticated = true

	log.Info("Logged in successfully.")
	return true, nil
}

/*
Logs you out of the client
*/
func (client *Client) Logout() (*http.Response, error) {
	return client.get("auth/logout", nil)
}

/*
Queries the client for the current application version
*/
func (client *Client) GetApplicationVersion() (string, error) {
	resp, err := client.get("app/version", nil)
	data, _ := io.ReadAll(resp.Body)
	return string(data), err
}

/*
Queries the client for the current Web API version
*/
func (client *Client) GetVersion() (string, error) {
	resp, err := client.get("app/webapiVersion", nil)
	data, _ := io.ReadAll(resp.Body)
	return string(data), err
}

/*
Gets the build info
*/
func (client *Client) GetBuildInfo() (string, error) {
	resp, err := client.get("app/buildInfo", nil)
	data, _ := io.ReadAll(resp.Body)
	return string(data), err
}

/*
Shuts down the client
*/
func (client *Client) Shutdown() (*http.Response, error) {
	return client.post("app/shutdown", nil)
}

/*
Gets current client preferences
*/
func (client *Client) GetPreferences() (map[string]interface{}, error) {
	resp, err := client.get("app/preferences", nil)
	byteValue, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	json.Unmarshal([]byte(byteValue), &data)
	return data, err
}

/*
Sets a preference

	key	string	The name of the preference
	value	string	The setting value
*/
func (client *Client) SetPreferences(token string, value string) (*http.Response, error) {
	params := map[string]string{
		"token": token,
		"value": value,
	}
	return client.post("app/setPreferences", params)
}

/*
Gets the default save path
*/
func (client *Client) GetDefaultSavePath() (string, error) {
	resp, err := client.get("app/defaultSavePath", nil)
	data, _ := io.ReadAll(resp.Body)
	return string(data), err
}

/*
Gets the main data

	rid int Response ID. Default is 0
*/
func (client *Client) GetMainData(rid string) (string, error) {
	params := map[string]string{
		"rid": rid,
	}
	resp, err := client.get("sync/maindata", params)
	data, _ := io.ReadAll(resp.Body)
	return string(data), err
}

/*
Gets torrent peers

	hash	string	Torrent hash
	rid	string	Response Id
*/
func (client *Client) GetTorrentPeers(hash string, rid string) (string, error) {
	params := make(map[string]string)
	params["hash"] = hash
	params["rid"] = rid
	resp, err := client.get("sync/torrentPeers", params)
	data, _ := io.ReadAll(resp.Body)
	return string(data), err
}

/*
Get transfer info
*/
func (client *Client) GetTransferInfo() (string, error) {
	resp, err := client.get("transfer/info", nil)
	data, _ := io.ReadAll(resp.Body)
	return string(data), err
}

/*
Check if alternative speeds are enabled

	0 means no
	1 means yes
*/
func (client *Client) GetSpeedLimitsMode() (string, error) {
	resp, err := client.get("transfer/speedLimitsMode", nil)
	data, _ := io.ReadAll(resp.Body)
	return string(data), err
}

/*
Gets the download speed limit value
*/
func (client *Client) GetDownloadLimit() (string, error) {
	resp, err := client.get("transfer/downloadLimit", nil)
	data, _ := io.ReadAll(resp.Body)
	return string(data), err
}

/*
Gets the upload speed limit value
*/
func (client *Client) GetUploadLimit() (string, error) {
	resp, err := client.get("transfer/uploadLimit", nil)
	data, _ := io.ReadAll(resp.Body)
	return string(data), err
}

/*
Ban peers

	peers	[]string	enter host:port values into an array
*/
func (client *Client) BanPeers(peers []string) (*http.Response, error) {
	params := client.processList("peers", peers)
	return client.post("transfer/uploadLimit", params)
}

/*
https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)#get-torrent-list

	Returns a list of torrents. Filtering is possible:

	filter	string 	Filter torrent list by state. Allowed state filters: all, downloading, seeding, completed, paused, active, inactive, resumed, stalled, stalled_uploading, stalled_downloading, errored
	category	string 	Get torrents with the given category (empty string means "without category"; no "category" parameter means "any category" <- broken until #11748 is resolved). Remember to URL-encode the category name. For example, My category becomes My%20category
	tag	string 	Get torrents with the given tag (empty string means "without tag"; no "tag" parameter means "any tag". Remember to URL-encode the category name. For example, My tag becomes My%20tag
	sort	string 	Sort torrents by given key. They can be sorted using any field of the response's JSON array (which are documented below) as the sort key.
	reverse	bool 	Enable reverse sorting. Defaults to false
	limit	integer 	Limit the number of torrents returned
	offset	integer 	Set offset (if less than 0, offset from end)
	hashes	string 	Filter by hashes. Can contain multiple hashes separated by |
*/
func (client *Client) GetTorrentList(filters map[string]string) ([]BasicTorrent, error) {
	var t []BasicTorrent
	resp, err := client.get("torrents/info", filters)
	json.NewDecoder(resp.Body).Decode(&t)
	return t, err
}

/*
Get the torrent details

	hash string Torrent hash value
*/
func (client *Client) GetTorrent(hash string) (Torrent, error) {
	params := map[string]string{
		"hash": hash,
	}
	var torrent Torrent
	resp, err := client.get("torrents/properties", params)

	json.NewDecoder(resp.Body).Decode(&torrent)
	return torrent, err
}

/*
Get torrent's tracker data

	hash	string Torrent hash value
*/
func (client *Client) GetTrackers(hash string) ([]Tracker, error) {
	params := map[string]string{
		"hash": hash,
	}
	var trackers []Tracker
	resp, err := client.get("torrents/trackers", params)

	json.NewDecoder(resp.Body).Decode(&trackers)
	return trackers, err
}

/*
Get torrent's webseeds data

	hash	string Torrent hash value
*/
func (client *Client) GetWebseeds(hash string) ([]WebSeed, error) {
	params := map[string]string{
		"hash": hash,
	}
	var webseeds []WebSeed
	resp, err := client.get("torrents/webseeds", params)

	json.NewDecoder(resp.Body).Decode(&webseeds)
	return webseeds, err
}

/*
Get torrent's files

	hash	string Torrent hash value
*/
func (client *Client) GetTorrentFiles(hash string) ([]TorrentFile, error) {
	params := map[string]string{
		"hash": hash,
	}
	var files []TorrentFile
	resp, err := client.get("torrents/files", params)

	json.NewDecoder(resp.Body).Decode(&files)
	return files, err
}

/*
Gets torrent's piece states

	0	Not downloaded yet
	1	Now downloading
	2	Already downloaded

	hash	string	Torrent hash value
*/
func (client *Client) GetTorrentPieceStates(hash string) ([]int, error) {
	params := map[string]string{
		"hash": hash,
	}
	var pieceStates []int
	resp, err := client.get("torrents/pieceStates", params)

	json.NewDecoder(resp.Body).Decode(&pieceStates)
	return pieceStates, err
}

/*
Get torrent's piece hashes

	hash  string Torrent hash value
*/
func (client *Client) GetTorrentPieceHashes(hash string) ([]int, error) {
	params := map[string]string{
		"hash": hash,
	}
	var pieceHashes []int
	resp, err := client.get("torrents/pieceHashes", params)

	json.NewDecoder(resp.Body).Decode(&pieceHashes)
	return pieceHashes, err
}

/*
Pause torrent

	hash string Torrent hash values
*/
func (client *Client) Pause(hash string) (*http.Response, error) {
	params := map[string]string{
		"hashes": hash,
	}
	return client.get("torrents/pause", params)
}

/*
Pause multiple torrents

	hash []string Torrent hash values in an array
*/
func (client *Client) PauseMultiple(hashes []string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	return client.get("torrents/pause", params)
}

/*
Pause all torrents
*/
func (client *Client) PauseAll() (*http.Response, error) {
	params := map[string]string{
		"hashes": "all",
	}
	return client.get("torrents/pause", params)
}

/*
Resume a torrent

	hash string Torrent hash value
*/
func (client *Client) Resume(hash string) (*http.Response, error) {
	params := map[string]string{
		"hashes": hash,
	}
	return client.get("torrents/resume", params)
}

/*
Resume multiple torrents

	hash []string Torrent hash values in an array
*/
func (client *Client) ResumeMultiple(hashes []string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	return client.get("torrents/resume", params)
}

/*
Resume all torrents
*/
func (client *Client) ResumeAll() (*http.Response, error) {
	params := map[string]string{
		"hashes": "all",
	}
	return client.get("torrents/resume", params)
}

/*
Delete a torrent

	hash string Torrent hash value
	deleteFiles	string false or true. Set if you want to delete the files and not just remove the torrent from the client.
*/
func (client *Client) Delete(hash string, deleteFiles string) (*http.Response, error) {
	params := map[string]string{
		"hashes":      hash,
		"deleteFiles": strings.ToLower(deleteFiles),
	}
	return client.get("torrents/delete", params)
}

/*
Delete multiple torrents

	hash string Torrent hash value
	deleteFiles	string false or true. Set if you want to delete the files and not just remove the torrent from the client.
*/
func (client *Client) DeleteMultiple(hashes []string, deleteFiles string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	params["deleteFiles"] = strings.ToLower(deleteFiles)
	return client.get("torrents/delete", params)
}

/*
Delete all torrents

	deleteFiles	string false or true. Set if you want to delete the files and not just remove the torrent from the client.
*/
func (client *Client) DeleteAll(deleteFiles string) (*http.Response, error) {
	params := map[string]string{
		"hashes":      "all",
		"deleteFiles": strings.ToLower(deleteFiles),
	}
	return client.get("torrents/delete", params)
}

/*
Recheck a torrent

	hash	string	Torrent hash value
*/
func (client *Client) Recheck(hash string) (*http.Response, error) {
	params := map[string]string{
		"hashes": hash,
	}
	return client.get("torrents/recheck", params)
}

/*
Recheck multiple torrents

	hash	[]string	Torrent hash values in an array
*/
func (client *Client) RecheckMultiple(hashes []string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	return client.get("torrents/recheck", params)
}

/*
Recheck all torrents
*/
func (client *Client) RecheckAll() (*http.Response, error) {
	params := map[string]string{
		"hashes": "all",
	}
	return client.get("torrents/recheck", params)
}

/*
Reannounce a torrent

	hash string Torrent hash value
*/
func (client *Client) Reannounce(hash string) (*http.Response, error) {
	params := map[string]string{
		"hashes": hash,
	}
	return client.get("torrents/reannounce", params)
}

/*
Reannounce multiple torrents

	hash []string	Torrent hash values in an array
*/
func (client *Client) ReannounceMultiple(hashes []string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	return client.get("torrents/reannounce", params)
}

/*
Reannounce all torrents
*/
func (client *Client) ReannounceAll() (*http.Response, error) {
	params := map[string]string{
		"hashes": "all",
	}
	return client.get("torrents/reannounce", params)
}

/*
Add trackers to torrent. Needs:

	hash	string
	urls	string Multiple allowed, separated by |
*/
func (client *Client) AddTracker(trackers map[string]string) (*http.Response, error) {
	return client.post("torrents/addTrackers", trackers)
}

/*
Edit torrent trackers. Needs:

	hash	string
	origUrl	string
	newUrl	string
*/
func (client *Client) EditTracker(trackers map[string]string) (*http.Response, error) {
	return client.post("torrents/editTracker", trackers)
}

/*
Remove trackers from torrent. Needs:

	hash	string	The hash of the torrent
	urls	string	URLs to remove, separated by |
*/
func (client *Client) RemoveTrackers(trackers map[string]string) (*http.Response, error) {
	return client.post("torrents/editTracker", trackers)
}

/*
Add peers to torrent

	hashes 	string 	The hash of the torrent, or multiple hashes separated by a pipe |
	peers 	string 	The peer to add, or multiple peers separated by a pipe |. Each peer is a colon-separated host:port
*/
func (client *Client) AddPeers(peers map[string]string) (*http.Response, error) {
	return client.post("torrents/addPeers", peers)
}

/*
Increase torrents' priority

	hashes	[]string	Torrent hash values in an array
*/
func (client *Client) IncreasePriority(hashes []string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	return client.post("torrents/increasePrio", params)
}

/*
Decrease torrents' priority

	hashes	[]string	Torrent hash values in an array
*/
func (client *Client) DecreasePriority(hashes []string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	return client.post("torrents/decreasePrio", params)
}

/*
Set torrents' priority to maximum

	hashes	[]string	Torrent hash values in an array
*/
func (client *Client) MaximumPriority(hashes []string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	return client.post("torrents/topPrio", params)
}

/*
Set torrents' priority to minimum

	hashes	[]string	Torrent hash values in an array
*/
func (client *Client) MinimumPriority(hashes []string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	return client.post("torrents/bottomPrio", params)
}

/*
Set torrent files' priority

	hash	string 	The hash of the torrent
	id	string 	File ids, separated by |
	priority	int 	File priority to set (consult torrent contents API for possible values)
*/
func (client *Client) SetFilePriority(params map[string]string) (*http.Response, error) {
	return client.post("torrents/filePrio", params)
}

/*
Get torrents download limit speed

	hashes	[]string	Torrent hash values in an array
*/
func (client *Client) GetTorrentDownloadLimit(hashes []string) (string, error) {
	params := client.processList("hashes", hashes)
	resp, _ := client.post("torrents/downloadLimit", params)
	data, _ := io.ReadAll(resp.Body)
	return string(data), nil
}

/*
Set torrents' download speed limit

	hashes	[]string	Torrent hash values in an array
	limit	string	Set download limit
*/
func (client *Client) SetTorrentDownloadLimit(hashes []string, limit string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	params["limit"] = limit
	return client.post("torrents/setDownloadLimit", params)
}

/*
Set torrents' share limits

	hashes can contain multiple hashes separated by | or set to all ratioLimit is the max ratio the torrent should be seeded until. -2 means the global limit should be used, -1 means no limit. seedingTimeLimit is the max amount of time (minutes) the torrent should be seeded. -2 means the global limit should be used, -1 means no limit.
*/
func (client *Client) SetTorrentShareLimit(hashes []string, ratioLimit string, seedingTimeLimit string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	params["ratioLimit"] = ratioLimit
	params["seedingTimeLimit"] = seedingTimeLimit
	return client.post("torrents/setShareLimits", params)
}

/*
Get torrents' upload speed limit

	hashes	[]string	Torrent hash values in an array
*/
func (client *Client) GetTorrentUploadLimit(hashes []string) (string, error) {
	params := client.processList("hashes", hashes)
	resp, _ := client.post("torrents/uploadLimit", params)
	data, _ := io.ReadAll(resp.Body)
	return string(data), nil
}

/*
Set torrents' upload speed limit

	hashes	[]string	Torrent hash values in an array
	limit	string	Set upload limit
*/
func (client *Client) SetTorrentUploadLimit(hashes []string, limit string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	params["limit"] = limit
	return client.post("torrents/setUploadLimit", params)
}

/*
Set torrents' save location

	hashes []string	Torrent hash value
	location string Save location
*/
func (client *Client) SetTorrentLocation(hashes []string, location string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	params["location"] = location
	return client.post("torrents/setLocation", params)
}

/*
Set torrent's name

	hash string Torrent hash value
	name	string	Torrent name
*/
func (client *Client) SetTorrentName(hash string, name string) (*http.Response, error) {
	params := map[string]string{
		"hash": hash,
		"name": name,
	}
	return client.post("torrents/rename", params)
}

/*
Set torrent category

	hashes []string Torrent hash values in an array
	category	string	Category name
*/
func (client *Client) SetTorrentCategory(hashes []string, category string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	params["category"] = category
	return client.post("torrents/setCategory", params)
}

/*
Get categories and their save paths
*/
func (client *Client) GetCategories() (map[string]interface{}, error) {
	resp, _ := client.post("torrents/categories", nil)
	byteValue, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	json.Unmarshal([]byte(byteValue), &data)
	return data, nil
}

/*
Create a category

	map:
		category	string Category name
		savePath	string	Save location
*/
func (client *Client) CreateCategory(params map[string]string) (*http.Response, error) {
	return client.post("torrents/createCategory", params)
}

/*
Edit a category

	map:
		category	string Category name
		savePath	string	Save location
*/
func (client *Client) EditCategory(params map[string]string) (*http.Response, error) {
	return client.post("torrents/editCategory", params)
}

/*
Remove a category

	category	string	Category name
*/
func (client *Client) RemoveCategory(category string) (*http.Response, error) {
	params := map[string]string{
		"category": category,
	}
	return client.post("torrents/removeCategories", params)
}

/*
Set torrents' tag

	hashes []string	Torrent hash values in an array
	tag	string	Tag name
*/
func (client *Client) SetTorrentTag(hashes []string, tag string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	params["tags"] = tag
	return client.post("torrents/addTags", params)
}

/*
Remove torrents' tag

	hashes []string	Torrent hash values in an array
	tag	string	Tag name
*/
func (client *Client) RemoveTorrentTag(hashes []string, tag string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	params["tags"] = tag
	return client.post("torrents/removeTags", params)
}

/*
Get available tags
*/
func (client *Client) GetTags() []interface{} {
	resp, _ := client.post("torrents/tags", nil)
	bytes, _ := io.ReadAll(resp.Body)
	var data []interface{}
	json.Unmarshal(bytes, &data)
	return data
}

/*
Create a tag

	tag	string	Tag name
*/
func (client *Client) CreateTag(tag string) (*http.Response, error) {
	params := map[string]string{
		"tags": tag,
	}
	return client.post("torrents/createTags", params)
}

/*
Delete a tag

	tag	string	Tag name
*/
func (client *Client) DeleteTag(tag string) (*http.Response, error) {
	params := map[string]string{
		"tags": tag,
	}
	return client.post("torrents/deleteTags", params)
}

/*
Set automatic torrent management (automatically set torrent's location to that of its category save location)

	hashes	[]string	Torrent hash values in an array
*/
func (client *Client) SetAutomaticTorrentManagement(hashes []string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	params["enable"] = "true"
	return client.post("torrents/setAutoManagement", params)
}

/*
Enable sequential download

	hash	string	Torrent hash value
*/
func (client *Client) SequentialDownload(hash string) (*http.Response, error) {
	params := map[string]string{
		"hashes": hash,
	}
	return client.post("torrents/toggleSequentialDownload", params)
}

/*
Enable sequential download for multiple torrents

	hash	[]string	Torrent hash values in an array
*/
func (client *Client) SequentialDownloadMultiple(hashes []string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	return client.post("torrents/toggleSequentialDownload", params)
}

/*
Enable sequential download for all torrents
*/
func (client *Client) SequentialDownloadAll() (*http.Response, error) {
	params := map[string]string{
		"hashes": "all",
	}
	return client.post("torrents/toggleSequentialDownload", params)
}

/*
Set first/last piece priority for a torrent

	hash	string	Torrent hash value
*/
func (client *Client) FirstLastPiecePriority(hash string) (*http.Response, error) {
	params := map[string]string{
		"hashes": hash,
	}
	return client.post("torrents/toggleFirstLastPiecePrio", params)
}

/*
Set first/last piece priority for multiple torrents

	hash	[]string	Torrent hash values in an array
*/
func (client *Client) FirstLastPiecePriorityMultiple(hashes []string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	return client.post("torrents/toggleFirstLastPiecePrio", params)
}

/*
Set first/last piece priority for all torrents
*/
func (client *Client) FirstLastPiecePriorityAll() (*http.Response, error) {
	params := map[string]string{
		"hashes": "all",
	}
	return client.post("torrents/toggleFirstLastPiecePrio", params)
}

/*
Set force start setting (true or false)

	hash string	Torrent hash value
	forceStart	string	"true" or "false"
*/
func (client *Client) SetForceStart(hash string, forceStart string) (*http.Response, error) {
	params := map[string]string{
		"hashes": hash,
		"value":  forceStart,
	}
	return client.post("torrents/setForceStart", params)
}

/*
Set force start setting (true or false) for multiple torrents

	hash []string	Torrent hash values in an array
	forceStart	string	"true" or "false"
*/
func (client *Client) SetForceStartMultiple(hashes []string, forceStart string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	params["value"] = forceStart
	return client.post("torrents/setForceStart", params)
}

/*
Set force start setting (true or false) for all torrents

	forceStart	string	"true" or "false"
*/
func (client *Client) SetForceStartAll(forceStart string) (*http.Response, error) {
	params := map[string]string{
		"hashes": "all",
		"value":  forceStart,
	}
	return client.post("torrents/setForceStart", params)
}

/*
Enable super seeding mode for a torrent

	hash string Torrent hash value
	superSeeding	string	"true" or "false"
*/
func (client *Client) SetSuperSeeding(hash string, superSeeding string) (*http.Response, error) {
	params := map[string]string{
		"hashes": hash,
		"value":  superSeeding,
	}
	return client.post("torrents/setSuperSeeding", params)
}

/*
Enable super seeding mode for multiple torrents

	hash []string Torrent hash values in an array
	superSeeding	string	"true" or "false"
*/
func (client *Client) SetSuperSeedingMultiple(hashes []string, superSeeding string) (*http.Response, error) {
	params := client.processList("hashes", hashes)
	params["value"] = superSeeding
	return client.post("torrents/setSuperSeeding", params)
}

/*
Enable super seeding mode for all torrents

	superSeeding	string	"true" or "false"
*/
func (client *Client) SetSuperSeedingAll(superSeeding string) (*http.Response, error) {
	params := map[string]string{
		"hashes": "all",
		"value":  superSeeding,
	}
	return client.post("torrents/setSuperSeeding", params)
}

/*
Rename a file

	hash	string	Torrent hash value
	oldPath	string	old file path
	newPath	string	new file path
*/
func (client *Client) RenameFile(hash string, oldPath string, newPath string) (*http.Response, error) {
	params := map[string]string{
		"hash":    hash,
		"oldPath": oldPath,
		"newPath": newPath,
	}
	return client.post("torrents/renameFile", params)
}

/*
Rename a folder

	hash	string	Torrent hash value
	oldPath	string	old file path
	newPath	string	new file path
*/
func (client *Client) RenameFolder(hash string, oldPath string, newPath string) (*http.Response, error) {
	params := map[string]string{
		"hash":    hash,
		"oldPath": oldPath,
		"newPath": newPath,
	}
	return client.post("torrents/renameFolder", params)
}
