
const hostEditDialog = document.querySelector('#dialog-host-edit')
const hostEditDialogTMPL = document.querySelector('#dialog-host-edit-template')
const noticeDialogTMPL = document.querySelector('#dialog-notice-template')
const noticeDialog = document.querySelector('#dialog-notice')

function openNewHost() {
    const tmpl = hostEditDialogTMPL.innerHTML
    hostEditDialog.innerHTML = tmpl.replaceAll("$$_TITLE_$$", "New host")

    hostEditDialog.showModal()
}

async function closeNewHost(e) {
    const name = document.querySelector("dialog input[name='name']")
    const address = document.querySelector("dialog input[name='address']")
    const port = document.querySelector("dialog input[name='port']")

    if (hostEditDialog.returnValue === 'confirm') {
        hostEditDialog.innerHTML = ""

        const tmpl = noticeDialogTMPL.innerHTML
        noticeDialog.innerHTML = tmpl.replaceAll("$$_MESSAGE_$$", "Done.")
        noticeDialog.showModal()
    }

    setTimeout(() => { noticeDialog.close() }, 2000)
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
