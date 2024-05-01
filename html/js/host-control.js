const hostEditDialog = document.querySelector('#dialog-host-edit')
const hostEditDialogTMPL = document.querySelector('#dialog-host-edit-template')
const noticeDialogTMPL = document.querySelector('#dialog-notice-template')
const noticeDialog = document.querySelector('#dialog-notice')


async function getHosts() {
    const tmpl = document.querySelector("#hosts-data-template").innerHTML
    const hostsContainer = document.querySelector("#hosts-data-container")

    const r = await fetch("/hosts?hosts-file=" + hostsFile)
    if (r.ok) {
        const response = await r.json()

        hostsContainer.innerHTML = ""
        response.forEach((el, i) => {
            line = tmpl
            line = line.replaceAll("$$_NAME_$$", el["name"])
            line = line.replaceAll("$$_ADDRESS_$$", el["address"])
            line = line.replaceAll("$$_PORT_$$", el["port"])
            line = line.replaceAll("$$_IDX_$$", i + 1)

            hostsContainer.innerHTML += line
        })

        hostsData = response
        return
    }
}

function setAuthType() {
    const selectedAuthName = hostEditDialog.querySelector("input[name='auth-type']:checked").id.replaceAll("use-", "")

    const authTypes = hostEditDialog.querySelectorAll("input[name='auth-type']")
    for (const atype of authTypes) {
        const authName = atype.id.replaceAll("use-", "")
        const inputEL = hostEditDialog.querySelector(`[name='${authName}']`)

        inputEL.parentElement.style.display = "none"
        inputEL.removeAttribute("required")
    }

    const inputEL = hostEditDialog.querySelector(`[name='${selectedAuthName}']`)
    inputEL.parentElement.style.display = "block"
    inputEL.setAttribute("required", "")
}

function moveKeyFilePathText() {
    const d = hostEditDialog
    d.querySelector("input#host-edit-private-key-file").value = d.querySelector("input#host-edit-private-key-file-browse").value
}

function openHostEditDialog(idxSTR = null) {
    const tmpl = hostEditDialogTMPL.innerHTML
    hostEditDialog.innerHTML = tmpl.replaceAll("$$_TITLE_$$", "New host")

    if (idxSTR) {
        const d = hostEditDialog
        const idx = parseInt(idxSTR) - 1
        d.querySelector("input#idx").value = idx

        const privateKeyFile = (hostsData[idx]["private-key-file"]) ? hostsData[idx]["private-key-file"] : ""
        const privateKeyText = (hostsData[idx]["private-key-text"]) ? hostsData[idx]["private-key-text"] : ""

        d.querySelector("input[name='name']").value = hostsData[idx]["name"]
        d.querySelector("input[name='address']").value = hostsData[idx]["address"]
        d.querySelector("input[name='port']").value = hostsData[idx]["port"]
        d.querySelector("input[name='username']").value = hostsData[idx]["username"]

        d.querySelector("input[name='password']").value = ""
        d.querySelector("input[name='password']").removeAttribute("required")

        d.querySelector("input[name='private-key-file']").value = privateKeyFile
        d.querySelector("textarea[name='private-key-text']").value = privateKeyText
        d.querySelector("textarea[name='description']").value = hostsData[idx]["description"]

        if (privateKeyFile == "" && privateKeyText == "") { }
        if (privateKeyFile != "") {
            d.querySelector("input#use-private-key-file").checked = true
        }
        if (privateKeyText != "") {
            d.querySelector("input#use-private-key-text").checked = true
        }
    }

    setAuthType()

    hostEditDialog.showModal()
}

async function saveHostData(e) {
    const d = e.target
    if (d.returnValue != 'confirm') {
        d.innerHTML = ""
        return
    }

    const idxSTR = d.querySelector("input#idx").value

    const name = d.querySelector("dialog input[name='name']").value
    const address = d.querySelector("dialog input[name='address']").value

    let port = d.querySelector("dialog input[name='port']").value.trim()
    if (!port) {
        port = 22
    }

    const username = d.querySelector("dialog input[name='username']").value

    let password, privateKeyFile, privateKeyText
    const selectedAuthName = hostEditDialog.querySelector("input[name='auth-type']:checked").id.replaceAll("use-", "")
    switch (selectedAuthName) {
        case "password":
            password = d.querySelector(`[name='${selectedAuthName}']`).value.trim()
            break
        case "private-key-file":
            privateKeyFile = d.querySelector(`[name='${selectedAuthName}']`).value
            break
        case "private-key-text":
            privateKeyText = d.querySelector(`[name='${selectedAuthName}']`).value
            break
    }

    const hostData = {
        name: name,
        address: address,
        port: parseInt(port),
        username: username,
        password: password,
        "private-key-file": privateKeyFile,
        "private-key-text": privateKeyText,
    }

    let params = `hosts-file=${hostsFile}`

    if (idxSTR) {
        const idx = parseInt(idxSTR)
        if (idx > -1) {
            params += `&idx=${idx}`
        }
    }

    const r = await fetch(`/hosts?${params}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(hostData)
    })

    let message = "failed to save."
    if (r.ok) {
        const response = await r.json()
        if (response.message == "success") {
            message = "done to save."
        }
    }

    hostEditDialog.innerHTML = ""
    const tmpl = noticeDialogTMPL.innerHTML
    noticeDialog.innerHTML = tmpl.replaceAll("$$_MESSAGE_$$", message)
    noticeDialog.showModal()
    getHosts()

    setTimeout(() => { noticeDialog.close() }, 2000)

    return
}

function cancelHostEditDialog() {
    hostEditDialog.innerHTML = ""
    hostEditDialog.close()
}

async function deleteHost(idxSTR) {
    const idx = parseInt(idxSTR) - 1

    const params = `hosts-file=${hostsFile}&idx=${idx}`
    const r = await fetch(`/hosts?${params}`, { method: "DELETE" })

    let message = "failed to delete."
    if (r.ok) {
        const response = await r.json()
        if (response.message == "success") {
            message = "done to save."
        }
    }

    const tmpl = noticeDialogTMPL.innerHTML
    noticeDialog.innerHTML = tmpl.replaceAll("$$_MESSAGE_$$", message)
    noticeDialog.showModal()
    getHosts()

    setTimeout(() => { noticeDialog.close() }, 2000)

    return
}

function closeNotice(e) {
    /* Close when click outside of dialog */
    // const target = e.target
    // const rect = target.getBoundingClientRect()
    // if (rect.left > e.clientX || rect.right < e.clientX ||
    //     rect.top > e.clientY || rect.bottom < e.clientY
    // ) {
    //     noticeDialog.close()
    // }

    noticeDialog.close()
}
