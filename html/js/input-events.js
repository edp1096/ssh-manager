function contextMenu(e) {
    console.log("TODO: context menu")
    return false
}

function preventCtrlWheel(e) {
    if (e.ctrlKey && e.deltaY != 0) { e.preventDefault() }
}

function preventDrag(e) {
    tagsAllow = ["BUTTON", "INPUT", "TEXTAREA"]
    if (tagsAllow.includes(e.target.tagName)) { return }
    e.preventDefault()
    return false
}

async function preventKeys(e) {
    // Function keys - F1 ~ F12
    if (e.code && e.code.startsWith("F")) {
        // for (let i = 1; i <= 11; i++) {
        for (let i = 1; i <= 12; i++) {
            if (e.code == `F${i}`) {
                e.preventDefault()
                break
            }
        }
    }

    // Ctrl
    if (e.ctrlKey) {
        if (e.ctrlKey && e.code == "KeyA") {
            return
        }
        if (e.ctrlKey && e.code == "KeyC") {
            return
        }
        if (e.ctrlKey && e.code == "Insert") {
            return
        }
        if (e.ctrlKey && e.code == "KeyX") {
            return
        }
        if (e.ctrlKey && e.code == "KeyV") {
            return
        }
        if (e.ctrlKey && e.code == "KeyW") {
            return
        }
        e.preventDefault()
    }
    // Alt
    if (e.altKey) {
        e.preventDefault()
    }
}
