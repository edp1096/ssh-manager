
const hostEditDialog = document.querySelector('#dialog-host-edit')
const hostEditDialogTMPL = document.querySelector('#dialog-host-edit-template')
const noticeDialogTMPL = document.querySelector('#dialog-notice-template')
const noticeDialog = document.querySelector('#dialog-notice')

function changeAuthType() {
    const selectedAuthName = hostEditDialog.querySelector("input[name='auth-type']:checked").id.replaceAll("use-", "")

    const authTypes = hostEditDialog.querySelectorAll("input[name='auth-type']")
    for (const atype of authTypes) {
        const authName = atype.id.replaceAll("use-", "")
        const inputEL = hostEditDialog.querySelector(`[name='${authName}']`)
        console.log(inputEL)
        inputEL.parentElement.style.display = "none"
        inputEL.removeAttribute("required")
    }

    const inputEL = hostEditDialog.querySelector(`[name='${selectedAuthName}']`)
    inputEL.parentElement.style.display = "block"
    inputEL.setAttribute("required", "")
}

function openNewHost() {
    const tmpl = hostEditDialogTMPL.innerHTML
    hostEditDialog.innerHTML = tmpl.replaceAll("$$_TITLE_$$", "New host")

    changeAuthType()

    hostEditDialog.showModal()
}

async function closeNewHost(e) {
    const name = document.querySelector("dialog input[name='name']")
    const address = document.querySelector("dialog input[name='address']")
    const port = document.querySelector("dialog input[name='port']")
    const username = document.querySelector("dialog input[name='username']")
    const password = document.querySelector("dialog input[name='password']")
    const privateKeyFile = document.querySelector("dialog input[name='private-key-file']")
    const privateKeyText = document.querySelector("dialog textarea")

    if (hostEditDialog.returnValue === 'confirm') {
        console.log(name.value)
        console.log(address.value)
        console.log(port.value)
        console.log(username.value)
        console.log(password.value)
        console.log(privateKeyFile.value)
        console.log(privateKeyText.value)

        hostEditDialog.innerHTML = ""

        const tmpl = noticeDialogTMPL.innerHTML
        noticeDialog.innerHTML = tmpl.replaceAll("$$_MESSAGE_$$", "Done.")
        noticeDialog.showModal()
    }

    setTimeout(() => { noticeDialog.close() }, 2000)
}

function cancelHostEditDialog() {
    hostEditDialog.innerHTML = ""
    hostEditDialog.close()

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
