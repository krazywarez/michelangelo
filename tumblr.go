package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://api.tumblr.com/v2"
const oauthBase = "https://www.tumblr.com"

type TumblrClient struct {
	ConsumerKey    string
	ConsumerSecret string
	Token          string
	TokenSecret    string
}

func NewClient(ck, cs, token, secret string) *TumblrClient {
	return &TumblrClient{ck, cs, token, secret}
}

// --- OAuth 1.0a ---

func oauthNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func oauthTimestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

func hmacSha1(key, data string) string {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *TumblrClient) oauthHeader(method, rawURL string, extraParams map[string]string) string {
	params := map[string]string{
		"oauth_consumer_key":     c.ConsumerKey,
		"oauth_nonce":            oauthNonce(),
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        oauthTimestamp(),
		"oauth_token":            c.Token,
		"oauth_version":          "1.0",
	}
	for k, v := range extraParams {
		params[k] = v
	}
	// Build base string
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(params[k]))
	}
	paramStr := strings.Join(parts, "&")
	baseStr := method + "&" + url.QueryEscape(rawURL) + "&" + url.QueryEscape(paramStr)
	sigKey := url.QueryEscape(c.ConsumerSecret) + "&" + url.QueryEscape(c.TokenSecret)
	params["oauth_signature"] = hmacSha1(sigKey, baseStr)

	// Build header
	headerParts := []string{}
	for k, v := range params {
		if strings.HasPrefix(k, "oauth_") {
			headerParts = append(headerParts, k+`="`+url.QueryEscape(v)+`"`)
		}
	}
	sort.Strings(headerParts)
	return "OAuth " + strings.Join(headerParts, ", ")
}

func (c *TumblrClient) get(endpoint string, params map[string]string) ([]byte, error) {
	u, _ := url.Parse(apiBase + endpoint)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	rawURL := apiBase + endpoint
	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("Authorization", c.oauthHeader("GET", rawURL, params))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *TumblrClient) post(endpoint string, params map[string]string) ([]byte, error) {
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	rawURL := apiBase + endpoint
	req, _ := http.NewRequest("POST", rawURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", c.oauthHeader("POST", rawURL, params))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// --- OAuth token exchange (no client tokens yet) ---

func oauthRequest(method, rawURL string, consumerKey, consumerSecret, token, tokenSecret string, params map[string]string) ([]byte, error) {
	c := &TumblrClient{consumerKey, consumerSecret, token, tokenSecret}
	if method == "POST" {
		return c.post(strings.TrimPrefix(rawURL, apiBase), params)
	}
	return c.get(strings.TrimPrefix(rawURL, apiBase), params)
}

func GetRequestToken(consumerKey, consumerSecret string) (string, string, error) {
	c := &TumblrClient{ConsumerKey: consumerKey, ConsumerSecret: consumerSecret}
	rawURL := oauthBase + "/oauth/request_token"
	req, _ := http.NewRequest("POST", rawURL, nil)
	req.Header.Set("Authorization", c.oauthHeader("POST", rawURL, nil))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	vals, _ := url.ParseQuery(string(body))
	return vals.Get("oauth_token"), vals.Get("oauth_token_secret"), nil
}

func GetAccessToken(consumerKey, consumerSecret, tmpToken, tmpSecret, verifier string) (string, string, error) {
	c := &TumblrClient{ConsumerKey: consumerKey, ConsumerSecret: consumerSecret, Token: tmpToken, TokenSecret: tmpSecret}
	rawURL := oauthBase + "/oauth/access_token"
	req, _ := http.NewRequest("POST", rawURL, strings.NewReader("oauth_verifier="+url.QueryEscape(verifier)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", c.oauthHeader("POST", rawURL, map[string]string{"oauth_verifier": verifier}))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	vals, _ := url.ParseQuery(string(body))
	return vals.Get("oauth_token"), vals.Get("oauth_token_secret"), nil
}

// --- API response types ---

type Post struct {
	ID         json.Number `json:"id"`
	IDString   string      `json:"id_string"`
	Type       string      `json:"type"`
	BlogName   string      `json:"blog_name"`
	PostURL    string      `json:"post_url"`
	ReblogKey  string      `json:"reblog_key"`
	NoteCount  int         `json:"note_count"`
	Liked      bool        `json:"liked"`
	Photos     []struct {
		OriginalSize struct {
			URL string `json:"url"`
		} `json:"original_size"`
	} `json:"photos"`
	VideoURL     string `json:"video_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Caption      string `json:"caption"`
}

type UserInfo struct {
	Response struct {
		User struct {
			Blogs []struct {
				Name string `json:"name"`
			} `json:"blogs"`
		} `json:"user"`
	}
}

type PostsResponse struct {
	Response struct {
		Posts []Post `json:"posts"`
	}
}

type TaggedResponse struct {
	Response []Post
}

// --- API methods ---

func (c *TumblrClient) GetUserInfo() (*UserInfo, error) {
	body, err := c.get("/user/info", nil)
	if err != nil {
		return nil, err
	}
	var result UserInfo
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *TumblrClient) GetDashboard(offset, limit int, postType string) ([]Post, error) {
	params := map[string]string{
		"offset": strconv.Itoa(offset),
		"limit":  strconv.Itoa(limit),
		"type":   postType,
	}
	body, err := c.get("/user/dashboard", params)
	if err != nil {
		return nil, err
	}
	var result PostsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w — body: %s", err, string(body))
	}
	return result.Response.Posts, nil
}

func (c *TumblrClient) GetBlogPosts(blogName string, offset, limit int, postType string) ([]Post, error) {
	params := map[string]string{
		"offset": strconv.Itoa(offset),
		"limit":  strconv.Itoa(limit),
		"type":   postType,
	}
	body, err := c.get("/blog/"+blogName+".tumblr.com/posts", params)
	if err != nil {
		return nil, err
	}
	var result PostsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse error: %w — body: %s", err, string(body))
	}
	return result.Response.Posts, nil
}

func (c *TumblrClient) GetTagged(tag string) ([]Post, error) {
	body, err := c.get("/tagged", map[string]string{"tag": tag})
	if err != nil {
		return nil, err
	}
	// Tagged endpoint returns array directly under response
	var raw struct {
		Response []Post `json:"response"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw.Response, nil
}

func (c *TumblrClient) LikePost(id, reblogKey string) error {
	_, err := c.post("/user/like", map[string]string{"id": id, "reblog_key": reblogKey})
	return err
}

func (c *TumblrClient) UnlikePost(id, reblogKey string) error {
	_, err := c.post("/user/unlike", map[string]string{"id": id, "reblog_key": reblogKey})
	return err
}

func (c *TumblrClient) ReblogPost(nativeBlog, id, reblogKey, sourceBlog string) error {
	_, err := c.post("/blog/"+nativeBlog+".tumblr.com/post/reblog", map[string]string{
		"id":         id,
		"reblog_key": reblogKey,
	})
	return err
}
