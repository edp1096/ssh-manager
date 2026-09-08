let hostsData = []
let hostsFile = "./hosts.dat"


async function connectSSH(categoryIdx, hostIdx, windowMode = null) {
    const categoryIndex = parseInt(categoryIdx)
    if (typeof categoryIndex != 'number' || !Number.isInteger(categoryIndex)) {
        await appDialogs.alert("Category index is not integer")
        return false
    }
    const hostIndex = parseInt(hostIdx)
    if (typeof hostIndex != 'number' || !Number.isInteger(hostIndex)) {
        await appDialogs.alert("Host index is not integer")
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
    await appDialogs.alert(await r.text(), { title: 'SSH connection failed' })
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
            await appDialogs.alert(response, { title: 'Could not unlock host list' })
            document.querySelector("#dialog-enter-password").showModal()
            return
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
    await appDialogs.alert(message, { title: 'Could not unlock host list' })

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
            await appDialogs.alert(response, { title: 'Password change failed' })
            document.querySelector("#dialog-change-password").showModal()
            d.querySelector("#change-password-old").value = passwordOld
            d.querySelector("#change-password-new").value = passwordNew
            return
        }

        const json = JSON.parse(response)
        if (json.message == "success") {
            await appDialogs.alert("The host file password has been changed.", { title: 'Password changed' })
            getHosts()
            return
        }
    }

    let message = "password change failed"
    try {
        message = await r.text()
    } catch (e) { }
    await appDialogs.alert(message, { title: 'Password change failed' })

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
        
        Version: ${version}
        <button type="button" class="repository-link" onclick="openRepository(this)" title="Open in browser">https://github.com/edp1096/ssh-manager</button>`.replace(/\n/g, "<br>")

        const noticeDialogTMPL = document.querySelector('#dialog-notice-template')
        const noticeDialog = document.querySelector('#dialog-notice')

        const tmpl = noticeDialogTMPL.innerHTML
        noticeDialog.innerHTML = tmpl.replaceAll("@@_MESSAGE_@@", message)
        noticeDialog.querySelector('h2').textContent = 'About SSH Manager'
        noticeDialog.showModal()
    }
}

async function openRepository(button) {
    if (button.disabled) { return }
    button.disabled = true
    try {
        const response = await fetch('/repository/open', { method: 'POST' })
        if (!response.ok) { throw new Error(await response.text()) }
    } catch (error) {
        await appDialogs.alert(error.message)
    } finally {
        button.disabled = false
    }
}

async function init() {
    initKeyboardNavigation()
    initPasswordVisibility()
    initWindowSizePersistence()
    document.addEventListener("keydown", preventKeys)
    document.addEventListener("mousedown", preventDrag)

    try {
        const response = await fetch("/terminal/status")
        if (response.ok) {
            const status = await response.json()
            if (!status.ready) await appDialogs.alert(status.message, { title: 'Terminal setup required' })
        }
    } catch (error) {
        console.error("Terminal status check failed", error)
    }
    document.querySelector("#dialog-enter-password").showModal()
    document.querySelector('#enter-password-input').focus({ preventScroll: true })
    window.addEventListener('focus', () => {
        const dialog = document.querySelector('#dialog-enter-password')
        if (dialog.open && !document.querySelector('.app-message-dialog[open]') && !dialog.contains(document.activeElement)) {
            dialog.querySelector('input').focus({ preventScroll: true })
        }
    })
}

function initWindowSizePersistence() {
    let timer
    let pendingSize = null
    let lastCaptured = null
    const captureSize = () => {
        if (document.visibilityState === 'hidden' || document.fullscreenElement) { return false }
        const width = Math.round(window.outerWidth)
        const height = Math.round(window.outerHeight)
        const x = Math.round(window.screenX)
        const y = Math.round(window.screenY)
        if (width < 320 || width > 16384 || height < 240 || height > 16384) { return false }
        if (!Number.isFinite(x) || !Number.isFinite(y) || Math.abs(x) > 65536 || Math.abs(y) > 65536) { return false }
        // Windows can report these off-screen coordinates while minimizing.
        if (x === -32000 || y === -32000) { return false }
        const size = JSON.stringify({ width, height, position: { x, y } })
        if (size === lastCaptured) { return false }
        lastCaptured = size
        pendingSize = size
        return true
    }
    const saveSize = () => {
        clearTimeout(timer)
        if (!pendingSize) { return }
        const size = pendingSize
        pendingSize = null
        fetch('/window-size', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: size,
            keepalive: true
        }).then(response => {
            if (!response.ok) { console.error('Window size could not be saved') }
        }).catch(error => console.error('Window size save failed', error))
    }
    const scheduleSave = () => {
        if (!captureSize()) { return }
        clearTimeout(timer)
        timer = setTimeout(saveSize, 250)
    }
    window.addEventListener('resize', scheduleSave)
    // Moving a window does not emit a resize event.
    setInterval(scheduleSave, 500)
    scheduleSave()
    window.addEventListener('pagehide', () => {
        captureSize()
        saveSize()
    })
    document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'hidden') { saveSize() }
    })
}

function initPasswordVisibility() {
    document.querySelectorAll('dialog').forEach(dialog => {
        dialog.addEventListener('close', () => {
            dialog.querySelectorAll('[data-password-toggle]').forEach(button => {
                const input = dialog.querySelector('#' + button.dataset.passwordToggle)
                input.type = 'password'
                updatePasswordToggle(button, false)
            })
        })
    })
}

function updatePasswordToggle(button, visible) {
    const label = visible ? 'Hide password' : 'Show password'
    button.title = label
    button.setAttribute('aria-label', label)
    button.setAttribute('aria-pressed', String(visible))
    button.querySelector('span').textContent = visible ? 'visibility_off' : 'visibility'
}

async function togglePasswordVisibility(button) {
    const dialog = button.closest('dialog')
    const input = dialog.querySelector('#' + button.dataset.passwordToggle)
    const visible = input.type === 'password'
    if (visible && input.id === 'host-edit-password' && !input.value) {
        const hostIndex = dialog.querySelector('#idx').value
        const originalAuth = dialog.querySelector('#auth-type-orig').value
        if (hostIndex !== '' && originalAuth === 'password') {
            button.disabled = true
            try {
                const params = new URLSearchParams({
                    'hosts-file': hostsFile,
                    'category-idx': dialog.querySelector('#category-idx').value,
                    'host-idx': hostIndex
                })
                const response = await fetch('/host-password?' + params, { cache: 'no-store' })
                if (!response.ok) { throw new Error('Could not load password') }
                const data = await response.json()
                if (!dialog.open || !input.isConnected) { return }
                if (!input.value) { input.value = data.password }
            } catch (error) {
                if (dialog.open && input.isConnected) { await appDialogs.alert(error.message) }
                return
            } finally { button.disabled = false }
        }
    }
    input.type = visible ? 'text' : 'password'
    updatePasswordToggle(button, visible)
}

document.addEventListener("DOMContentLoaded", () => { init() })
