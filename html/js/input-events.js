function contextMenu(e) {
    console.log("TODO: context menu")
    return false
}

function preventCtrlWheel(e) {
    if (e.ctrlKey && e.deltaY != 0) { e.preventDefault() }
}

async function preventKeys(e) {
    // Function keys - F1 ~ F12
    if (e.code && e.code.startsWith("F")) {
        // for (let i = 1; i <= 12; i++) {
        for (let i = 1; i <= 11; i++) {
            if (e.code == `F${i}`) {
                e.preventDefault()
                break
            }
        }
    }

    // Ctrl
    if (e.ctrlKey) {
        if (e.ctrlKey && e.code == "KeyC") {
            return
        }
        if (e.ctrlKey && e.code == "Insert") {
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
