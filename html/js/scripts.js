let hostsData = []
let hostsFile = "./hosts.dat"

function preventDrag(e) { e.preventDefault() }

async function connectSSH(idx, windowMode = null) {
    const hostsIndex = parseInt(idx)
    if (typeof hostsIndex != 'number' || !Number.isInteger(hostsIndex)) {
        alert("Index is not integer")
        return false
    }

    let params = (windowMode) ? windowMode : ""

    const body = { "hosts-file": hostsFile, "index": hostsIndex }
    const r = await fetch(`/session/open?window-mode=${windowMode}`, {
        method: "POST",
        headers: new Headers({}),
        body: JSON.stringify(body)
    })

    if (r.ok) {
        const response = await r.text()
        document.querySelector("#result").innerHTML = response
        return
    }
}

document.addEventListener("DOMContentLoaded", () => { getHosts() })
