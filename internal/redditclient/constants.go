package redditclient

const (
	ANDROID_CLIENT_ID             = "ohXpoqrZYub1kg"
	CONTENT_WARNING_ACCEPT_COOKIE = "_options=%7B%22pref_quarantine_optin%22%3A%20true%2C%20%22pref_gated_sr_optin%22%3A%20true%7D"
)

// Android app versions for User-Agent spoofing
var androidVersions = []string{
	"Reddit/2023.46.0/Android 12",
	"Reddit/2023.45.0/Android 11",
	"Reddit/2023.44.0/Android 13",
	"Reddit/2023.43.0/Android 12",
	"Reddit/2023.42.0/Android 11",
}
