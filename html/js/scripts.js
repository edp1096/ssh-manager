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

async function enterPassword() {
    const d = document.querySelector("#dialog-enter-password")

    const password = d.querySelector("#enter-password-input").value.trim()
    const body = { "password": password }
    const r = await fetch(`/enter-password?hosts-file=${hostsFile}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body)
    })

    if (r.ok) {
        const response = await r.text()

        let isJSON = true
        try {
            JSON.parse(response)
        } catch (e) {
            isJSON = false
        }

        if (!isJSON) {
            alert(response)
            document.querySelector("#dialog-enter-password").showModal()
        }

        const json = JSON.parse(response)
        if (json.message == "success") {
            getHosts()
            return
        }
    }

    alert("incorrect password")
    document.querySelector("#dialog-enter-password").showModal()
    return
}

function init() {
    document.querySelector("#dialog-enter-password").showModal()
}

document.addEventListener("DOMContentLoaded", () => { init() })
