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
        response.forEach((el, i) => {
            line = tmpl
            line = line.replaceAll("$$_NAME_$$", el["name"])
            line = line.replaceAll("$$_ADDRESS_$$", el["address"])
            line = line.replaceAll("$$_PORT_$$", el["port"])
            line = line.replaceAll("$$_IDX_$$", i + 1)

            hostsContainer.innerHTML += line
        })

        return
    }
}

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
