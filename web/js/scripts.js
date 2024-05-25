let hostsData = []
let hostsFile = "./hosts.dat"


async function connectSSH(categoryIdx, hostIdx, windowMode = null) {
    const categoryIndex = parseInt(categoryIdx)
    if (typeof categoryIndex != 'number' || !Number.isInteger(categoryIndex)) {
        alert("Category index is not integer")
        return false
    }
    const hostIndex = parseInt(hostIdx)
    if (typeof hostIndex != 'number' || !Number.isInteger(hostIndex)) {
        alert("Host index is not integer")
        return false
    }

    let modeWindow = (windowMode) ? windowMode : ""

    const body = { "hosts-file": hostsFile, "category-index": categoryIndex, "host-index": hostIndex }
    const r = await fetch(`/session/open?window-mode=${modeWindow}`, {
        method: "POST",
        headers: new Headers({}),
        body: JSON.stringify(body)
    })

    if (r.ok) {
        const response = await r.text()
        // console.log(response)
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

    let message = "incorrect password"
    if (password == "") {
        message = "Empty password input"
    }
    alert(message)

    d.querySelector("#enter-password-input").value = ""
    document.querySelector("#dialog-enter-password").showModal()
    return
}

function openChangePasswordDialog() {
    document.querySelector("#dialog-change-password").showModal()
}

function cancelChangePasswordDialog() {
    const d = document.querySelector("#dialog-change-password")
    d.returnValue = "cancel"
    d.close()
}

async function changeHostFilePassword(e) {
    const d = e.target
    if (d.returnValue != 'confirm') {
        d.querySelector("#change-password-old").value = ""
        d.querySelector("#change-password-new").value = ""
        return
    }

    const passwordOld = d.querySelector("#change-password-old").value.trim()
    const passwordNew = d.querySelector("#change-password-new").value.trim()
    d.querySelector("#change-password-old").value = ""
    d.querySelector("#change-password-new").value = ""

    const body = { "password-old": passwordOld, "password-new": passwordNew }
    const r = await fetch(`/host-file-password?hosts-file=${hostsFile}`, {
        method: "PUT",
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
            document.querySelector("#dialog-change-password").showModal()
            d.querySelector("#change-password-old").value = passwordOld
            d.querySelector("#change-password-new").value = passwordNew
        }

        const json = JSON.parse(response)
        if (json.message == "success") {
            alert("Password of host file is changed")
            getHosts()
            return
        }
    }

    let message = "password change failed"
    try {
        message = await r.text()
    } catch (e) { }
    alert(message)

    d.querySelector("#change-password-old").value = ""
    d.querySelector("#change-password-new").value = ""
    document.querySelector("#dialog-change-password").showModal()
    return
}

async function getApplicationVersion() {
    const r = await fetch("/version")
    if (r.ok) {
        const version = await r.text()
        const message = `SSH Manager
        
        Ver. ${version}`.replace(/\n/g, "<br>")

        const noticeDialogTMPL = document.querySelector('#dialog-notice-template')
        const noticeDialog = document.querySelector('#dialog-notice')

        const tmpl = noticeDialogTMPL.innerHTML
        noticeDialog.innerHTML = tmpl.replaceAll("@@_MESSAGE_@@", message)
        noticeDialog.showModal()
    }
}

function init() {
    // const urlParams = new URLSearchParams(window.location.search)
    // const systemOS = urlParams.get("system-os")
    // if (systemOS == "windows") {
    //     document.querySelectorAll(".tmux").forEach((e) => { e.style.display = "none" })
    // } else {
    // }
    document.querySelectorAll("template").forEach((t) => {
        t.content.querySelectorAll(".wt").forEach((e) => {
            e.remove()
        })
    })

    document.addEventListener("keydown", preventKeys)
    document.addEventListener("mousedown", preventDrag)

    document.querySelector("#dialog-enter-password").showModal()
}

document.addEventListener("DOMContentLoaded", () => { init() })
