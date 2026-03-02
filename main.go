package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var tmpl = template.Must(template.ParseFiles("templates/layout.html"))

func main() {
	http.HandleFunc("/", handleGallery)
	http.HandleFunc("/blog/", handleBlog)
	http.HandleFunc("/search", handleSearch)
	http.HandleFunc("/auth/callback", handleAuthCallback)
	http.HandleFunc("/logout", handleLogout)

	// JSON API endpoints (called by frontend JS)
	http.HandleFunc("/api/dashboard", handleAPIDashboard)
	http.HandleFunc("/api/blog/", handleAPIBlog)
	http.HandleFunc("/api/search", handleAPISearch)
	http.HandleFunc("/api/like", handleAPILike)
	http.HandleFunc("/api/reblog", handleAPIReblog)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Michelangelo running on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// --- Auth helpers ---

func getTokens(r *http.Request) (token, secret string) {
	c1, err1 := r.Cookie("perm_token")
	c2, err2 := r.Cookie("perm_secret")
	if err1 != nil || err2 != nil {
		return "", ""
	}
	return c1.Value, c2.Value
}

func requireAuth(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	token, secret := getTokens(r)
	if token == "" || secret == "" {
		startOAuth(w, r)
		return "", "", false
	}
	return token, secret, true
}

func startOAuth(w http.ResponseWriter, r *http.Request) {
	consumerKey := os.Getenv("CONSUMER_KEY")
	consumerSecret := os.Getenv("CONSUMER_SECRET")
	tmpToken, tmpSecret, err := GetRequestToken(consumerKey, consumerSecret)
	if err != nil {
		http.Error(w, "OAuth init failed: "+err.Error(), 500)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "tmp_token", Value: tmpToken, Path: "/", HttpOnly: true})
	http.SetCookie(w, &http.Cookie{Name: "tmp_secret", Value: tmpSecret, Path: "/", HttpOnly: true})
	http.Redirect(w, r, "https://www.tumblr.com/oauth/authorize?oauth_token="+tmpToken, http.StatusFound)
}

func handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	verifier := r.URL.Query().Get("oauth_verifier")
	if verifier == "" {
		http.Error(w, "Missing oauth_verifier", 400)
		return
	}
	tmpToken, err1 := r.Cookie("tmp_token")
	tmpSecret, err2 := r.Cookie("tmp_secret")
	if err1 != nil || err2 != nil {
		http.Error(w, "Missing temporary tokens", 400)
		return
	}
	consumerKey := os.Getenv("CONSUMER_KEY")
	consumerSecret := os.Getenv("CONSUMER_SECRET")
	permToken, permSecret, err := GetAccessToken(consumerKey, consumerSecret, tmpToken.Value, tmpSecret.Value, verifier)
	if err != nil {
		http.Error(w, "OAuth exchange failed: "+err.Error(), 500)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "perm_token", Value: permToken, Path: "/", HttpOnly: true})
	http.SetCookie(w, &http.Cookie{Name: "perm_secret", Value: permSecret, Path: "/", HttpOnly: true})
	http.Redirect(w, r, "/", http.StatusFound)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "perm_token", Value: "", MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: "perm_secret", Value: "", MaxAge: -1, Path: "/"})
	http.Redirect(w, r, "/", http.StatusFound)
}

// --- Page handlers (render shell, JS does the loading) ---

type PageData struct {
	Title     string
	BlogName  string
	QueryOrBlog string
	View      string // "dashboard" | "blog" | "search"
}

func handleGallery(w http.ResponseWriter, r *http.Request) {
	token, secret, ok := requireAuth(w, r)
	if !ok {
		return
	}
	client := NewClient(os.Getenv("CONSUMER_KEY"), os.Getenv("CONSUMER_SECRET"), token, secret)
	info, err := client.GetUserInfo()
	blogName := ""
	if err == nil && len(info.Response.User.Blogs) > 0 {
		blogName = info.Response.User.Blogs[0].Name
	}
	tmpl.Execute(w, PageData{Title: "Michelangelo", BlogName: blogName, View: "dashboard"})
}

func handleBlog(w http.ResponseWriter, r *http.Request) {
	_, _, ok := requireAuth(w, r)
	if !ok {
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/blog/")
	name = strings.Trim(name, "/")
	if name == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	tmpl.Execute(w, PageData{Title: name + " — Michelangelo", QueryOrBlog: name, View: "blog"})
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	_, _, ok := requireAuth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query().Get("q")
	tmpl.Execute(w, PageData{Title: "Search — Michelangelo", QueryOrBlog: q, View: "search"})
}

// --- API handlers (JSON, consumed by frontend) ---

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, `{"error":%q}`, msg)
}

func getClient(w http.ResponseWriter, r *http.Request) (*TumblrClient, bool) {
	token, secret, ok := requireAuth(w, r)
	if !ok {
		return nil, false
	}
	return NewClient(os.Getenv("CONSUMER_KEY"), os.Getenv("CONSUMER_SECRET"), token, secret), true
}

func handleAPIDashboard(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(w, r)
	if !ok {
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	postType := r.URL.Query().Get("type")
	if postType == "" {
		postType = "photo"
	}
	posts, err := client.GetDashboard(offset, 20, postType)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func handleAPIBlog(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(w, r)
	if !ok {
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/blog/")
	name = strings.Trim(name, "/")
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	postType := r.URL.Query().Get("type")
	if postType == "" {
		postType = "photo"
	}
	posts, err := client.GetBlogPosts(name, offset, 20, postType)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func handleAPISearch(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(w, r)
	if !ok {
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		jsonError(w, "missing query", 400)
		return
	}
	posts, err := client.GetTagged(q)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(posts)
}

func handleAPILike(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(w, r)
	if !ok {
		return
	}
	id := r.URL.Query().Get("id")
	key := r.URL.Query().Get("key")
	unlike := r.URL.Query().Get("unlike") == "1"
	var err error
	if unlike {
		err = client.UnlikePost(id, key)
	} else {
		err = client.LikePost(id, key)
	}
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok":true}`)
}

func handleAPIReblog(w http.ResponseWriter, r *http.Request) {
	client, ok := getClient(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		jsonError(w, "bad request", 400)
		return
	}
	blogName := r.FormValue("blog_name")
	id := r.FormValue("id")
	key := r.FormValue("reblog_key")
	nativeBlog := r.FormValue("native_blog")
	err := client.ReblogPost(nativeBlog, id, key, blogName)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"ok":true}`)
}
