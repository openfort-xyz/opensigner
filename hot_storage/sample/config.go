package main

import "os"

var authServerURL = os.Getenv("AUTH_SERVER_URL") // e.g. "https://auth.example.com/validate"
