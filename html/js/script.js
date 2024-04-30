let hostsData
let hostsFile = "./hosts.dat"

async function getHosts() {
    const tmpl = document.querySelector("#hosts-data-template").innerHTML
    const hostsContainer = document.querySelector("#hosts-data-container")

    const r = await fetch("/hosts?hosts-file=" + hostsFile)
    if (r.ok) {
        const response = await r.json()
        console.log(response)

        hostsContainer.innerHTML = ""
        response.forEach(el => {
            line = tmpl
            line = line.replace("$$_NAME_$$", el["name"])
            line = line.replace("$$_ADDRESS_$$", el["address"])
            line = line.replace("$$_PORT_$$", el["port"])

            hostsContainer.innerHTML += line
        })

        return
    }
}

async function connectSSH(idx) {
    const hostsIndex = parseInt(idx)
    if (typeof hostsIndex != 'number' || !Number.isInteger(hostsIndex)) {
        alert("Index is not integer")
        return false
    }

    const body = { "hosts-file": hostsFile, "index": hostsIndex }
    const r = await fetch("/session/open", {
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

const newHostDialog = document.querySelector('#dialog-host-edit')
const noticeDialogTMPL = document.querySelector('#dialog-notice-template')
const noticeDialog = document.querySelector('#dialog-notice')

function openNewHost() { newHostDialog.showModal() }

async function closeNewHost(e) {
    const name = document.querySelector("dialog input[name='name']")
    const address = document.querySelector("dialog input[name='address']")
    const port = document.querySelector("dialog input[name='port']")

    if (newHostDialog.returnValue === 'confirm') {
        noticeDialog.innerHTML = noticeDialogTMPL.innerHTML
        noticeDialog.innerHTML.replace("$$_MESSAGE_$$", "완료")
        noticeDialog.showModal()
    }

    setTimeout(() => { noticeDialog.close() }, 2000)
}

function closeNotice(e) {
    const target = e.target
    const rect = target.getBoundingClientRect()
    if (rect.left > e.clientX || rect.right < e.clientX ||
        rect.top > e.clientY || rect.bottom < e.clientY
    ) {
        noticeDialog.close()
    }
}

function closingCode() {
    const r = fetch("/quit")
    return false
}

globalThis.onunload = () => { closingCode() }
globalThis.addEventListener("unload", () => { closingCode() })
document.addEventListener("DOMContentLoaded", () => { getHosts() })
