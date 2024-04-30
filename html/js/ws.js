const watchdogURI = `ws://${serverURI}/connection-watchdog`
const wsockWatchdog = new WebSocket(watchdogURI)

wsockWatchdog.onopen = function () {
    console.log('wsock watchdog connected')
}
