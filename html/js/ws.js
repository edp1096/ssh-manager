const watchdogURI = `ws://${serverURI}/connection-watchdog`
const wsockWatchdog = new WebSocket(watchdogURI)

function wsSendMessage(message) { wsockWatchdog.send(message) }
function sendDocumentTitle() { wsSendMessage(`document title|${document.title}`) }

wsockWatchdog.onopen = function () {
    console.log('wsock watchdog connected')
    sendDocumentTitle()
}
