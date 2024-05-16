const currentURL = new URL(window.location.href)
const serverURI = `${currentURL.hostname}:${currentURL.port}`

const dialogCloseWaitTime = 5000