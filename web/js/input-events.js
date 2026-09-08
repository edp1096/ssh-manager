function contextMenu(e) {
    console.log("TODO: context menu")
    return false
}

function preventCtrlWheel(e) {
    if (e.ctrlKey && e.deltaY != 0) { e.preventDefault() }
}

function preventDrag(e) {
    if (e.target.closest('a[href], button, input, textarea, select, label, [tabindex]')) { return }
    e.preventDefault()
    return false
}

async function preventKeys(e) {
    if (e.isComposing || e.target.closest('input, textarea, select, [contenteditable="true"]')) { return }
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
            const tagsAllow = ["INPUT", "TEXTAREA"]
            if (tagsAllow.includes(e.target.tagName)) { return }
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

let lastListFocus = null
let lastPageFocus = null

function listRows() {
    return [...document.querySelectorAll('.category-name, .category.active > .host-part-info')]
}

function focusListRow(row) {
    if (!row) { return }
    document.querySelectorAll('.category-name, .host-part-info').forEach(item => {
        item.tabIndex = item === row ? 0 : -1
    })
    row.focus()
    row.scrollIntoView({ block: 'nearest' })
}

function setCategoryExpanded(category, expanded) {
    category.classList.toggle('active', expanded)
    const heading = category.querySelector('.category-name')
    heading.setAttribute('aria-expanded', String(expanded))
    if (!expanded && category.querySelector('.host-part-info[tabindex="0"]')) {
        category.querySelectorAll('.host-part-info').forEach(row => { row.tabIndex = -1 })
        heading.tabIndex = 0
        if (category.contains(document.activeElement)) { focusListRow(heading) }
    }
}

function restoreListNavigation() {
    const rows = listRows()
    const saved = lastListFocus
    const row = rows.find(item => saved?.id && item.dataset.hostId === saved.id)
        || rows.find(item => item.dataset.category === saved?.category && item.dataset.host === saved?.host)
        || rows.find(item => item.dataset.category === saved?.category)
        || rows[0]
    document.querySelectorAll('.category-name, .host-part-info').forEach(item => {
        item.tabIndex = item === row ? 0 : -1
    })
    return row
}

function initKeyboardNavigation() {
    const container = document.querySelector('#hosts-data-container')
    document.addEventListener('focusin', event => {
        if (event.target.closest('dialog')) { return }
        lastPageFocus = event.target
        const row = event.target.closest('.host-part-info, .category-name')
            || event.target.closest('.category')?.querySelector('.category-name')
        if (!row) { return }
        lastListFocus = { category: row.dataset.category, host: row.dataset.host, id: row.dataset.hostId }
        document.querySelectorAll('.category-name, .host-part-info').forEach(item => {
            item.tabIndex = item === row ? 0 : -1
        })
    })
    container.addEventListener('click', event => {
        if (event.target.closest('button')) { return }
        focusListRow(event.target.closest('.host-part-info')
            || event.target.closest('.category')?.querySelector('.category-name'))
    })
    document.addEventListener('keydown', event => {
        if (container.hidden) { return }
        if (event.isComposing || event.altKey || event.metaKey) { return }
        if (event.target.closest('dialog, input, textarea, select, [contenteditable="true"]')) { return }
        if (document.querySelector('#order-container').style.display === 'block') { return }
        const arrowKey = ['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(event.key)
        const plainArrow = arrowKey && !event.ctrlKey && !event.shiftKey
        const row = event.target.closest('.host-part-info, .category-name')
            || event.target.closest('.category')?.querySelector('.category-name')
        if (!row) {
            if (plainArrow && event.target.closest('.button-group-left, .button-group-right')) {
                const previousRow = restoreListNavigation()
                if (previousRow) {
                    event.preventDefault()
                    focusListRow(previousRow)
                }
            }
            return
        }
        if (event.target !== row) {
            if (!plainArrow) { return }
            focusListRow(row)
        }
        if (event.ctrlKey || event.shiftKey) {
            if (event.key === 'Enter' && row.matches('.host-part-info')) {
                event.preventDefault()
                if (!event.repeat) {
                    if (event.ctrlKey) openFileBrowser(row.dataset.category, row.dataset.host)
                    else connectSSH(row.dataset.category, row.dataset.host, null)
                }
            }
            return
        }
        const rows = listRows()
        const index = rows.indexOf(row)
        const category = row.closest('.category')
        const heading = category.querySelector('.category-name')
        switch (event.key) {
            case 'ArrowDown': focusListRow(rows[Math.min(index + 1, rows.length - 1)]); break
            case 'ArrowUp': focusListRow(rows[Math.max(index - 1, 0)]); break
            case 'Home': focusListRow(rows[0]); break
            case 'End': focusListRow(rows[rows.length - 1]); break
            case 'ArrowRight':
                if (row === heading) {
                    if (category.classList.contains('active')) { focusListRow(category.querySelector('.host-part-info')) }
                    else { setCategoryExpanded(category, true) }
                }
                break
            case 'ArrowLeft':
                if (row !== heading) { focusListRow(heading) }
                else { setCategoryExpanded(category, false) }
                break
            case 'Enter':
            case ' ':
                if (event.repeat) { break }
                if (row === heading) { setCategoryExpanded(category, !category.classList.contains('active')) }
                else if (event.key === 'Enter') { connectSSH(row.dataset.category, row.dataset.host, 'new_window') }
                break
            default: return
        }
        event.preventDefault()
    })
    document.querySelectorAll('dialog').forEach(dialog => {
        dialog.addEventListener('cancel', event => {
            if (dialog.id === 'dialog-enter-password') { event.preventDefault() }
            else { dialog.returnValue = 'cancel' }
        })
        dialog.addEventListener('close', () => {
            requestAnimationFrame(() => {
                if (document.querySelector('dialog[open]')) { return }
                if (lastPageFocus?.isConnected && lastPageFocus.getClientRects().length) { lastPageFocus.focus() }
                else { focusListRow(restoreListNavigation()) }
            })
        })
    })
}
