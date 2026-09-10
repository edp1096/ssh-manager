const fileBrowser = (() => {
    const views = new Map()
    let polling = false
    let skipCloseConfirmation = false
    let queueRevision = 0
    let drag = null
    let menu = null
    function closeMenu(restore = false) {
        if (!menu) return
        const { node, focus } = menu
        menu = null; node.remove()
        if (restore && focus?.isConnected && focus.getClientRects().length) focus.focus()
    }
    document.addEventListener('pointerdown', event => { if (!menu?.node.contains(event.target)) closeMenu() })
    document.addEventListener('scroll', event => {
        // Blurring a long path input can scroll its text without moving the menu anchor.
        if (!event.target.matches?.('input, textarea')) closeMenu()
    }, true)
    window.addEventListener('resize', () => closeMenu())
    document.addEventListener('keydown', event => {
        if (event.isComposing || event.altKey || event.metaKey || event.shiftKey) return
        if (!(event.key === 'F5' && !event.ctrlKey) && !(event.ctrlKey && event.code === 'KeyR')) return
        const view = [...views.values()].find(view => !view.closed && !view.content.hidden)
        if (!view) return
        event.preventDefault(); event.stopImmediatePropagation()
        if (event.repeat || view.refreshing || !view.session || document.querySelector('dialog[open]')) return
        view.refreshing = true
        Promise.all(['local', 'remote'].map(side => load(view, side, view[side].path)))
            .finally(() => { view.refreshing = false })
    }, true)
    function createQueue(view) {
        view.jobs = new Map()
        view.queue = element('details', undefined, 'file-queue')
        view.queue.hidden = true
        view.summary = element('summary')
        view.jobList = element('div', undefined, 'file-queue-list')
        view.queue.append(view.summary, view.jobList, button('Clear finished', async () => {
            try {
                await api('/files/jobs?' + new URLSearchParams({ session: view.session }), 'DELETE')
                startPolling()
            } catch (error) { view.summary.textContent = error.message }
        }))
        return view.queue
    }

    function element(tag, text, className) {
        const node = document.createElement(tag)
        if (text !== undefined) node.textContent = text
        if (className) node.className = className
        return node
    }
    function button(text, handler) {
        const node = element('button', text)
        node.type = 'button'
        node.addEventListener('click', handler)
        return node
    }
    async function api(url, method = 'GET', body, signal) {
        const response = await fetch(url, {
            method, signal,
            headers: body ? { 'Content-Type': 'application/json' } : {},
            body: body ? JSON.stringify(body) : undefined,
        })
        const data = await response.json()
        if (!response.ok) throw new Error(data.error || `Request failed (${response.status})`)
        return data
    }
    function size(value) {
        if (value < 1024) return `${value} B`
        if (value < 1048576) return `${(value / 1024).toFixed(1)} KiB`
        return `${(value / 1048576).toFixed(1)} MiB`
    }
    function modifiedTime(value) {
        const date = new Date(value)
        if (!value || !Number.isFinite(date.getTime()) || value.startsWith('0001-')) return '—'
        const pad = value => String(value).padStart(2, '0')
        return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
    }
    function status(view, message, error = false) {
        view.status.textContent = message
        view.status.dataset.error = String(error)
        view.status.hidden = !message || !error
    }
    function pane(view, side) {
        const node = element('section', undefined, 'file-pane')
        const heading = element('header')
        heading.append(element('strong', side === 'local' ? 'Local' : `Remote · ${view.protocol}`))
        const input = element('input')
        input.setAttribute('aria-label', `${side} directory`)
        const location = element('form', undefined, 'file-path')
        const list = element('div', undefined, 'file-list')
        const message = element('div', undefined, 'file-status')
        message.setAttribute('role', 'status')
        const state = { node, input, list, message, entries: [], selected: new Set(), path: '', parent: '', home: '', sequence: 0 }
        list.tabIndex = 0
        list.setAttribute('aria-label', `${side} files`)
        list.addEventListener('focusin', event => {
            const row = event.target.closest('tbody tr')
            if (row) {
                state.focusedRow = row
                list.tabIndex = -1
                for (const item of list.querySelectorAll('tbody tr, tbody button')) item.tabIndex = -1
                row.tabIndex = 0
            } else if (event.target === list && !state.loading) {
                const first = list.querySelector('.file-selected, [data-file-path]')
                if (first) focusEntry(state, first, { ctrlKey: true })
            }
        })
        list.addEventListener('keydown', event => {
            const toggle = event.ctrlKey && (event.code === 'Space' || event.key === ' ')
            if ((event.isComposing && !toggle) || event.altKey || event.metaKey || event.target.closest('thead')) return
            if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
                event.preventDefault(); event.stopPropagation()
                const destination = event.key === 'ArrowLeft' ? 'local' : 'remote'
                if (destination === side || view.closed || !view.session) return
                const target = view[destination]
                const remembered = target.focusedRow
                const row = !target.loading && (remembered?.isConnected && target.list.contains(remembered)
                    ? remembered : target.list.querySelector('.file-selected, [data-file-path]')
                        || target.list.querySelector('[data-navigation=".."]:has(button:not(:disabled))'))
                if (row) focusEntry(target, row, { ctrlKey: event.ctrlKey || target.selected.size > 0 })
                else target.list.focus()
                return
            }
            if (toggle) {
                event.preventDefault(); event.stopPropagation()
                const row = event.target.closest('[data-file-path]')
                if (row && !event.repeat && !state.loading && !view.closed && view.session)
                    selectEntry(state, row.dataset.filePath, { ctrlKey: true })
                return
            }
            if (event.key === 'F2' && !event.ctrlKey && !event.shiftKey) {
                event.preventDefault(); event.stopPropagation()
                if (!event.repeat) operate(view, side, 'rename')
                return
            }
            if (event.key === 'Delete' && !event.ctrlKey && !event.shiftKey) {
                event.preventDefault(); event.stopPropagation()
                if (!event.repeat && state.selected.size) operate(view, side, 'delete')
                return
            }
            const movement = ['ArrowUp', 'ArrowDown', 'PageUp', 'PageDown', 'Home', 'End'].includes(event.key)
            if (movement || ['Enter', 'Backspace'].includes(event.key)) {
                event.preventDefault(); event.stopPropagation()
                if (state.loading || view.closed || !view.session) return
                const current = event.target.closest('tr')
                if (movement) {
                    const rows = [...list.querySelectorAll('[data-file-path], [data-navigation]')]
                    if (!rows.length) return
                    const index = rows.indexOf(current)
                    const down = event.key === 'ArrowDown' || event.key === 'PageDown'
                    let nextIndex = index < 0 ? (down ? 0 : rows.length - 1) : index + (down ? 1 : -1)
                    if (event.key === 'Home') nextIndex = 0
                    else if (event.key === 'End') nextIndex = rows.length - 1
                    else if (index >= 0 && (event.key === 'PageUp' || event.key === 'PageDown')) {
                        const height = Math.max(1, list.clientHeight - (list.querySelector('thead')?.offsetHeight || 0))
                        const origin = rows[index].getBoundingClientRect().top
                        nextIndex = index
                        while (nextIndex + (down ? 1 : -1) >= 0 && nextIndex + (down ? 1 : -1) < rows.length) {
                            nextIndex += down ? 1 : -1
                            if (Math.abs(rows[nextIndex].getBoundingClientRect().top - origin) >= height) break
                        }
                    }
                    const next = rows[Math.max(0, Math.min(rows.length - 1, nextIndex))]
                    focusEntry(state, next, event)
                } else if (!event.repeat && !event.ctrlKey && !event.shiftKey) {
                    if (event.key === 'Enter' && state.selected.size > 1) {
                        enqueue(view, side)
                    } else if (event.key === 'Backspace' || current?.dataset.navigation === '..') {
                        if (state.parent && state.parent !== state.path) load(view, side, state.parent)
                    } else {
                        const entry = state.entries.find(e => e.path === current?.dataset.filePath)
                        if (entry?.directory || entry?.link) load(view, side, entry.path)
                        else if (entry) enqueue(view, side, { paths: [entry.path] })
                    }
                }
                return
            }
            if (event.key.toLowerCase() === 'a' && event.ctrlKey) {
                event.preventDefault(); event.stopPropagation()
                state.selected = new Set(state.entries.map(e => e.path)); updateSelection(state)
            } else if (event.key === 'Escape') {
                event.preventDefault(); event.stopPropagation()
                state.selected.clear(); state.anchor = null; state.navigationSelected = null; updateSelection(state)
            }
        })
        list.addEventListener('click', event => {
            if (!event.target.closest('tr')) { state.selected.clear(); state.anchor = null; state.navigationSelected = null; updateSelection(state) }
        })
        list.addEventListener('contextmenu', event => {
            event.preventDefault(); event.stopPropagation()
            if (state.loading || !view.session) return
            const row = event.target.closest('[data-file-path]')
            if (event.target.closest('.file-navigation-row')) return
            if (row && !state.selected.has(row.dataset.filePath)) selectEntry(state, row.dataset.filePath, {})
            if (!row) { state.selected.clear(); updateSelection(state) }
            showMenu(view, side, event.clientX, event.clientY, row || list)
        })
        const dropTarget = event => {
            if (!drag || drag.view !== view || drag.side === side || view.closed || state.loading
                || view[drag.side].sequence !== drag.sequence || !state.path || event.target.closest('.file-navigation-row')) return null
            const row = event.target.closest('[data-file-path]')
            const entry = state.entries.find(e => e.path === row?.dataset.filePath)
            return entry?.directory && !entry.link ? entry.path : state.path
        }
        list.addEventListener('dragover', event => {
            event.preventDefault()
            const destination = dropTarget(event)
            event.dataTransfer.dropEffect = destination ? 'copy' : 'none'
            list.classList.toggle('file-drop-target', !!destination)
        })
        list.addEventListener('dragleave', event => { if (!list.contains(event.relatedTarget)) list.classList.remove('file-drop-target') })
        list.addEventListener('drop', event => {
            event.preventDefault(); event.stopPropagation(); list.classList.remove('file-drop-target')
            const destination = dropTarget(event)
            const transfer = drag; drag = null
            if (destination) enqueue(view, transfer.side, { paths: transfer.paths, destination })
            else status(view, 'Drag between the local and remote lists in this tab. External file drops are not supported.')
        })
        const home = button('', () => load(view, side, state.home))
        home.title = 'Home'; home.setAttribute('aria-label', `${side} home directory`)
        const homeIcon = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
        homeIcon.setAttribute('viewBox', '0 0 24 24'); homeIcon.setAttribute('class', 'ui-icon'); homeIcon.setAttribute('aria-hidden', 'true')
        const homePath = document.createElementNS('http://www.w3.org/2000/svg', 'path')
        homePath.setAttribute('d', 'M3 10l9-7 9 7M5 9v12h5v-7h4v7h5V9')
        homeIcon.append(homePath); home.append(homeIcon)
        const refresh = button('↻', () => load(view, side, state.path))
        refresh.title = 'Refresh (F5 / Ctrl+R)'; refresh.setAttribute('aria-label', `Refresh ${side} files`)
        const up = button('↑', () => load(view, side, state.parent))
        up.title = 'Parent folder (Backspace in file list)'; up.setAttribute('aria-label', `${side} parent directory`)
        location.append(home, refresh, up, input)
        location.addEventListener('submit', event => { event.preventDefault(); load(view, side, input.value) })
        node.append(heading, location, list, message)
        return state
    }
    async function load(view, side, path) {
        if (!view.session || view.closed) return
        closeMenu()
        const p = view[side]
        const sequence = ++p.sequence
        p.loading = true
        p.message.textContent = 'Loading…'
        p.message.dataset.error = 'false'
        p.list.setAttribute('aria-busy', 'true')
        try {
            const data = await api(`/files/sessions/${view.session}/entries?${new URLSearchParams({ side, path })}`, 'GET', undefined, view.controller.signal)
            if (view.closed || sequence !== p.sequence) return
            const hadFocus = p.list.contains(document.activeElement)
            const focusedPath = document.activeElement.closest('[data-file-path]')?.dataset.filePath
            const previousPath = p.path
            p.path = data.path; p.parent = data.parent; p.input.value = data.path
            p.entries = data.entries; p.selected.clear(); p.anchor = null; p.navigationSelected = null
            render(view, side)
            if (hadFocus) {
                const rows = [...p.list.querySelectorAll('[data-file-path]')]
                const preferred = previousPath === data.path ? focusedPath : previousPath
                const row = rows.find(row => row.dataset.filePath === preferred) || rows[0]
                    || p.list.querySelector('[data-navigation=".."]:has(button:not(:disabled))')
                if (row) focusEntry(p, row, {})
                else p.list.focus()
            }
            p.message.textContent = `${data.entries.length} entries`
        } catch (error) {
            if (!view.closed && sequence === p.sequence) {
                p.message.textContent = error.message
                p.message.dataset.error = 'true'
            }
        } finally {
            if (sequence === p.sequence) { p.loading = false; p.list.removeAttribute('aria-busy') }
        }
    }
    function focusEntry(p, row, event) {
        if (row.dataset.filePath) {
            if (!event.ctrlKey || event.shiftKey) selectEntry(p, row.dataset.filePath, event)
        } else if (!event.ctrlKey && !event.shiftKey) {
            p.selected.clear(); p.anchor = null; p.navigationSelected = row.dataset.navigation; updateSelection(p)
        }
        row.focus({ preventScroll: true })
        // Keep the cursor below the sticky column header while scrolling.
        const listRect = p.list.getBoundingClientRect(), rect = row.getBoundingClientRect()
        const top = listRect.top + (p.list.querySelector('thead')?.getBoundingClientRect().height || 0)
        if (rect.top < top) p.list.scrollTop -= top - rect.top
        else if (rect.bottom > listRect.bottom) p.list.scrollTop += rect.bottom - listRect.bottom
    }
    function updateSelection(p) {
        for (const row of p.list.querySelectorAll('[data-navigation]')) {
            const selected = !p.selected.size && row.dataset.navigation === p.navigationSelected
            row.classList.toggle('file-selected', selected)
            row.setAttribute('aria-selected', String(selected))
        }
        for (const row of p.list.querySelectorAll('[data-file-path]')) {
            const selected = p.selected.has(row.dataset.filePath)
            row.classList.toggle('file-selected', selected)
            row.setAttribute('aria-selected', String(selected))
            row.querySelector('input').checked = selected
        }
        const all = p.list.querySelector('thead input')
        if (all) { all.checked = !!p.entries.length && p.selected.size === p.entries.length; all.indeterminate = p.selected.size > 0 && !all.checked }
    }
    function selectEntry(p, path, event) {
        p.navigationSelected = null
        const order = p.order || []
        if (event.shiftKey && order.includes(p.anchor)) {
            if (!event.ctrlKey) p.selected.clear()
            const a = order.indexOf(p.anchor), b = order.indexOf(path)
            order.slice(Math.min(a, b), Math.max(a, b) + 1).forEach(key => p.selected.add(key))
        } else {
            if (event.ctrlKey) { if (p.selected.has(path)) p.selected.delete(path); else p.selected.add(path) }
            else { p.selected.clear(); p.selected.add(path) }
            p.anchor = path
        }
        updateSelection(p)
    }
    function showMenu(view, side, x, y, focus) {
        closeMenu()
        const p = view[side], node = element('div', undefined, 'file-context-menu')
        node.setAttribute('role', 'menu')
        const add = (title, action, enabled = true) => {
            const item = button(title, () => { closeMenu(true); action() })
            item.title = title
            item.setAttribute('role', 'menuitem'); item.disabled = !enabled; node.append(item)
        }
        const entry = p.entries.find(e => e.path === [...p.selected][0])
        const opensFolder = p.selected.size === 1 && !!(entry?.directory || entry?.link)
        add('Open folder' + (opensFolder ? ' (Enter)' : ''), () => load(view, side, entry.path), opensFolder)
        add((side === 'local' ? 'Upload' : 'Download') + (p.selected.size > 0 && !opensFolder ? ' (Enter)' : ''), () => enqueue(view, side), p.selected.size > 0)
        add('Rename (F2)', () => operate(view, side, 'rename'), p.selected.size === 1)
        add('Delete (Delete)', () => operate(view, side, 'delete'), p.selected.size > 0)
        add('New folder', () => operate(view, side, 'mkdir'))
        add('Refresh (F5 / Ctrl+R)', () => load(view, side, p.path))
        document.body.append(node); menu = { node, focus }
        const bounds = node.getBoundingClientRect()
        node.style.left = `${Math.max(0, Math.min(x, innerWidth - bounds.width))}px`
        node.style.top = `${Math.max(0, Math.min(y, innerHeight - bounds.height))}px`
        node.addEventListener('contextmenu', event => { event.preventDefault(); event.stopPropagation() })
        node.addEventListener('keydown', event => {
            if (event.key === 'Escape' || event.key === 'Tab') { event.preventDefault(); event.stopPropagation(); closeMenu(true); return }
            const items = [...node.querySelectorAll('button:not(:disabled)')]
            let index = items.indexOf(document.activeElement)
            if (event.key === 'ArrowDown') index = (index + 1) % items.length
            else if (event.key === 'ArrowUp') index = (index - 1 + items.length) % items.length
            else return
            event.preventDefault(); event.stopPropagation(); items[index].focus()
        })
        node.querySelector('button:not(:disabled)')?.focus({ preventScroll: true })
    }
    function render(view, side) {
        const p = view[side]
        p.list.tabIndex = 0
        const table = element('table')
        const head = element('thead'), headings = element('tr')
        const all = element('input'); all.type = 'checkbox'; all.tabIndex = -1; all.setAttribute('aria-label', 'Select all entries')
        const first = element('th'); first.append(all)
        headings.append(first)
        for (const [key, title] of [['name', 'Name'], ['size', 'Size'], ['modified', 'Timestamp']]) {
            const th = element('th')
            th.className = 'file-sort-header'
            th.setAttribute('aria-sort', (p.sort || 'name') === key ? (p.desc ? 'descending' : 'ascending') : 'none')
            const sort = () => {
                p.desc = (p.sort || 'name') === key ? !p.desc : false; p.sort = key; render(view, side)
                p.list.querySelector(`[data-sort="${key}"]`)?.focus()
            }
            const control = button('', event => { event.stopPropagation(); sort() })
            control.append(element('span', title))
            const arrow = element('span', (p.sort || 'name') === key ? (p.desc ? '↓' : '↑') : '', 'file-sort-arrow')
            arrow.setAttribute('aria-hidden', 'true')
            control.append(arrow)
            control.dataset.sort = key
            th.append(control)
            th.addEventListener('click', sort)
            headings.append(th)
        }
        head.append(headings)
        const body = element('tbody')
        // Navigation rows are not filesystem entries: keep them above sorting
        // Their visual selection is separate from transferable file selections.
        for (const marker of ['.', '..']) {
            const row = element('tr', undefined, 'file-navigation-row')
            row.dataset.navigation = marker
            row.tabIndex = -1
            row.addEventListener('click', event => { if (!p.loading) focusEntry(p, row, event) })
            const nameCell = element('td')
            const name = marker === '.' ? element('span', '.')
                : button('..', () => {})
            name.className = 'file-name'
            name.title = marker === '.' ? `Current folder: ${p.path}` : `Parent folder: ${p.parent}`
            if (marker === '..') {
                name.tabIndex = -1
                name.setAttribute('aria-label', 'Open parent folder')
                name.disabled = p.parent === p.path
                row.addEventListener('dblclick', () => { if (p.parent !== p.path) load(view, side, p.parent) })
            }
            nameCell.append(name)
            row.append(element('td'), nameCell, element('td', '—'), element('td', '—'))
            body.append(row)
        }
        const entries = [...p.entries].sort((a, b) => Number(b.directory) - Number(a.directory)
            || ((p.sort === 'size' ? a.size - b.size : p.sort === 'modified'
                ? (Date.parse(a.modified) || 0) - (Date.parse(b.modified) || 0)
                : a.name.localeCompare(b.name, undefined, { numeric: true })) * (p.desc ? -1 : 1)))
        p.order = entries.map(e => e.path)
        for (const entry of entries) {
            const row = element('tr'), checkCell = element('td'), nameCell = element('td')
            row.dataset.filePath = entry.path; row.tabIndex = -1; row.draggable = true
            row.addEventListener('mousedown', event => {
                if (event.button !== 0 || event.target.closest('input') || event.ctrlKey || event.shiftKey || p.loading) return
                if (!p.selected.has(entry.path)) selectEntry(p, entry.path, {})
            })
            row.addEventListener('click', event => {
                if (!event.target.closest('input') && !p.loading) {
                    selectEntry(p, entry.path, event)
                    row.focus({ preventScroll: true })
                }
            })
            row.addEventListener('dblclick', event => {
                if (event.target.closest('input') || p.loading) return
                if (entry.directory || entry.link) load(view, side, entry.path)
                else enqueue(view, side, { paths: [entry.path] })
            })
            row.addEventListener('keydown', event => {
                if (event.target.closest('input') || event.ctrlKey || event.altKey || event.metaKey || event.isComposing) return
                if (event.key === ' ') {
                    event.preventDefault(); event.stopPropagation()
                    if (!event.repeat && !p.loading && !view.closed) selectEntry(p, entry.path, event)
                }
            })
            row.addEventListener('dragstart', event => {
                if (p.loading || event.target.closest('input')) { event.preventDefault(); return }
                closeMenu()
                if (!p.selected.has(entry.path)) selectEntry(p, entry.path, {})
                drag = { view, side, sequence: p.sequence, paths: [...p.selected] }
                event.dataTransfer.effectAllowed = 'copy'
                event.dataTransfer.setData('application/x-ssh-manager-files', 'internal')
            })
            row.addEventListener('dragend', () => { drag = null; document.querySelectorAll('.file-drop-target').forEach(n => n.classList.remove('file-drop-target')) })
            const check = element('input'); check.type = 'checkbox'; check.tabIndex = -1
            check.checked = p.selected.has(entry.path)
            check.setAttribute('aria-label', `Select ${entry.name}`)
            check.addEventListener('change', () => {
                if (check.checked) p.selected.add(entry.path); else p.selected.delete(entry.path)
                p.anchor = entry.path; updateSelection(p)
            })
            checkCell.append(check)
            const name = element('span', `${entry.directory ? '📁 ' : entry.link ? '↗ ' : ''}${entry.name}`)
            name.className = 'file-name'
            name.title = `${entry.name}\n${modifiedTime(entry.modified)}`
            nameCell.append(name)
            row.append(checkCell, nameCell, element('td', entry.directory ? '—' : size(entry.size)),
                element('td', modifiedTime(entry.modified), 'file-modified'))
            body.append(row)
        }
        all.addEventListener('change', () => {
            p.selected.clear()
            body.querySelectorAll('input:not(:disabled)').forEach(check => { check.checked = all.checked })
            if (all.checked) p.entries.forEach(e => p.selected.add(e.path))
            updateSelection(p)
        })
        table.append(head, body)
        p.list.replaceChildren(table)
        updateSelection(p)
    }
    async function enqueue(view, side, drop) {
        const source = view[side], target = view[side === 'local' ? 'remote' : 'local']
        if (!view.session || source.loading || target.loading || !target.path || view.enqueuing) return
        const paths = drop?.paths || [...source.selected]
        if (!paths.length) { status(view, 'Select one or more files or folders first.'); return }
        const batch = crypto.randomUUID()
        view.enqueuing = true
        const destination = drop?.destination || target.path
        try {
            for (const path of paths) {
                if (view.closed) break
                await api('/files/jobs', 'POST', {
                    session: view.session, direction: side === 'local' ? 'upload' : 'download',
                    source: path, destination, interactive: true, batch,
                }, view.controller.signal)
                startPolling()
            }
            view.queue.open = true
            status(view, 'Files added to the transfer queue.')
        } catch (error) { if (!view.closed) status(view, error.message, true) }
        finally { view.enqueuing = false }
    }
    function renameInput(view, value) {
        if (document.querySelector('#file-rename-dialog')) return Promise.resolve(null)
        return new Promise(resolve => {
            const previous = document.activeElement
            const dialog = element('dialog'); dialog.id = 'file-rename-dialog'
            const title = element('h2', 'Rename'); title.id = 'file-rename-title'
            dialog.setAttribute('aria-labelledby', title.id)
            const form = element('form'), label = element('label', 'New name'), input = element('input')
            input.type = 'text'; input.value = value; input.required = true
            input.autocomplete = 'off'; input.spellcheck = false
            label.append(input)
            let composing = false, result = null
            input.addEventListener('compositionstart', () => { composing = true })
            input.addEventListener('compositionend', () => { composing = false })
            dialog.addEventListener('keydown', event => {
                // IME owns text input. Its Enter commits text, not the rename operation.
                if (event.key === 'Enter' && event.target === input) {
                    event.preventDefault()
                    if (!composing && !event.isComposing && event.keyCode !== 229 && !event.repeat) form.requestSubmit()
                }
                event.stopPropagation()
            })
            form.addEventListener('submit', event => {
                event.preventDefault()
                if (!composing && input.value) { result = input.value; dialog.close() }
            })
            const submit = element('button', 'Rename', 'ok'); submit.type = 'submit'
            submit.title = 'Rename (Enter in name field)'
            const actions = element('div', undefined, 'file-rename-actions')
            const cancel = button('Cancel', () => dialog.close()); cancel.title = 'Cancel (Esc)'
            actions.append(cancel, submit)
            form.append(label, actions)
            dialog.append(title, form); document.body.append(dialog)
            const abort = () => dialog.close()
            view.controller.signal.addEventListener('abort', abort, { once: true })
            dialog.addEventListener('close', () => {
                view.controller.signal.removeEventListener('abort', abort)
                dialog.remove()
                if (previous?.isConnected) previous.focus({ preventScroll: true })
                resolve(result)
            }, { once: true })
            dialog.showModal(); input.focus(); input.select()
        })
    }
    async function operate(view, side, action) {
        const p = view[side]
        if (!view.session || view.closed || p.loading || p.operating || !p.path) return
        let paths = [...p.selected], name
        if (action === 'mkdir') {
            name = await appDialogs.prompt('Create a folder in the current directory.'); if (!name || view.closed) return
            paths = [p.path]
        } else if (action === 'rename') {
            if (paths.length !== 1) { status(view, 'Select exactly one entry to rename.'); return }
            name = await renameInput(view, p.entries.find(e => e.path === paths[0]).name)
            if (!name || view.closed) return
        } else {
            if (!paths.length) { status(view, 'Select entries to delete.'); return }
            if (!await appDialogs.confirm(`Permanently delete ${paths.length} selected entry(s)? Folders and ALL contents will be deleted. This cannot be undone. Symbolic links are deleted without following them.\n\n${paths.join('\n')}`, { title: 'Delete selected entries', confirmText: 'Delete', danger: true }) || view.closed) return
        }
        const recursivePaths = new Set(paths.filter(path => p.entries.find(e => e.path === path)?.directory))
        p.operating = true
        try {
            for (const path of paths) await api(`/files/sessions/${view.session}/operation`, 'POST', {
                side, action, path, name,
                recursive: action === 'delete' && recursivePaths.has(path),
            }, view.controller.signal)
            status(view, `${action === 'mkdir' ? 'Folder created' : action === 'rename' ? 'Renamed' : 'Deleted'}.`)
        } catch (error) { if (!view.closed) status(view, error.message, true) }
        finally { p.operating = false; await load(view, side, p.path) }
    }
    function startPolling() {
        queueRevision++
        if (polling) return
        polling = true
        poll()
    }
    function showConflict(view, job) {
        if (document.querySelector('dialog[open]') || view.content.hidden || view.closed || view.enqueuing) return
        const conflict = job.conflict
        if (view.resolvedConflict === conflict.id) return
        const dialog = element('dialog'); dialog.id = 'file-conflict-dialog'
        dialog.dataset.conflictId = conflict.id
        dialog.dataset.jobId = job.id
        const title = element('h2', 'File already exists'); title.id = 'file-conflict-title'
        dialog.setAttribute('aria-labelledby', title.id)
        const intro = element('p', 'Choose how to handle the existing file.')
        const details = element('div', undefined, 'file-conflict-details')
        for (const [label, entry] of [['Incoming file', conflict.source], ['Existing file', conflict.destination]]) {
            const section = element('section')
            section.append(element('strong', label), element('p', entry.path, 'file-conflict-path'),
                element('p', `${size(entry.size)} · ${modifiedTime(entry.modified)}`))
            details.append(section)
        }
        const label = element('label', 'Apply to '), scope = element('select')
        for (const [value, title] of [['file', 'This file only'], ['batch', 'This transfer'], ['session', 'This connection tab']]) {
            const option = element('option', title); option.value = value; scope.append(option)
        }
        label.append(scope)
        const note = element('p', 'Connection choices reset when this tab closes. Cancel stops this transfer, including its queued files.', 'file-conflict-note')
        const error = element('p', '', 'file-status'); error.setAttribute('role', 'status'); error.dataset.error = 'true'; error.hidden = true
        const actions = element('footer', undefined, 'file-conflict-actions')
        let sending = false
        const resolve = async action => {
            if (sending) return
            sending = true
            for (const control of dialog.querySelectorAll('button, select')) control.disabled = true
            try {
                await api(`/files/jobs/${job.id}/resolve`, 'POST', { conflictId: conflict.id, action, scope: scope.value })
                view.resolvedConflict = conflict.id
                dialog.remove(); startPolling()
            } catch (err) {
                error.textContent = err.message; error.hidden = false
                sending = false
                for (const control of dialog.querySelectorAll('button, select')) control.disabled = false
            }
        }
        const overwrite = button('Overwrite', () => resolve('overwrite')); overwrite.className = 'ok'
        actions.append(button('Cancel transfer', () => resolve('cancel')), button('Skip', () => resolve('skip')), overwrite)
        dialog.append(title, intro, details, label, note, error, actions)
        dialog.addEventListener('cancel', event => { event.preventDefault(); resolve('cancel') })
        const previous = document.activeElement
        const abort = () => dialog.remove()
        view.controller.signal.addEventListener('abort', abort, { once: true })
        // Polling removes stale dialogs too (e.g. cancellation from the queue).
        const observer = new MutationObserver(() => {
            if (dialog.isConnected) return
            observer.disconnect(); view.controller.signal.removeEventListener('abort', abort)
            if (previous?.isConnected && !view.closed) previous.focus({ preventScroll: true })
        })
        observer.observe(document.body, { childList: true })
        document.body.append(dialog); dialog.showModal()
        actions.querySelectorAll('button')[1].focus()
    }
    async function poll() {
        try {
            const revision = queueRevision
            const data = await api('/files/jobs') || []
            const openConflict = document.querySelector('#file-conflict-dialog')
            if (openConflict && !data.some(j => j.status === 'waiting' && j.conflict?.id === openConflict.dataset.conflictId)) openConflict.remove()
            let active = 0
            for (const view of views.values()) {
                const { queue, summary, jobList, jobs } = view
                const ownJobs = data.filter(job => job.session === view.session)
                let ownActive = 0
                const present = new Set()
                for (const job of ownJobs) {
                    present.add(job.id)
                    const busy = ['queued', 'running', 'waiting'].includes(job.status)
                    if (job.status === 'waiting' && job.conflict) showConflict(view, job)
                    if (busy) { active++; ownActive++ }
                    let row = jobs.get(job.id)
                    if (!row) {
                        const node = element('div', undefined, 'file-job')
                        const label = element('span', undefined, 'file-job-label')
                        const progress = element('progress')
                        const cancel = button('Cancel', async () => {
                            try { await api(`/files/jobs/${job.id}`, 'DELETE') }
                            catch (error) { label.textContent = error.message }
                        })
                        const retry = button('Retry', async () => {
                            try { await api('/files/jobs', 'POST', { session: job.session, direction: job.direction, source: job.source, destination: job.destination, interactive: true }); startPolling() }
                            catch (error) { label.textContent = error.message }
                        })
                        node.append(label, progress, cancel, retry); jobList.append(node)
                        row = { node, label, progress, cancel, retry }; jobs.set(job.id, row)
                    }
                    if (row.status !== job.status && !busy) {
                        if (view) { const side = job.direction === 'upload' ? 'remote' : 'local'; load(view, side, view[side].path) }
                    }
                    row.status = job.status; row.node.dataset.status = job.status
                    row.label.textContent = `${job.direction === 'upload' ? '↑' : '↓'} ${job.source} → ${job.destination} · ${job.status} · ${size(job.bytes)}${job.skipped ? ` · ${job.skipped} skipped` : ''}${job.error ? ` · ${job.error}` : ''}`
                    row.progress.max = job.total || 1; row.progress.value = job.bytes
                    row.progress.setAttribute('aria-label', `Transfer ${job.source}`)
                    row.cancel.hidden = !busy
                    row.retry.hidden = !['failed', 'cancelled'].includes(job.status)
                }
                for (const [id, row] of jobs) if (!present.has(id)) { row.node.remove(); jobs.delete(id) }
                queue.hidden = !ownJobs.length
                summary.textContent = `Transfers · ${ownActive} active · ${ownJobs.length} total`
            }
            if (active || revision !== queueRevision || [...views.values()].some(v => v.enqueuing)) setTimeout(poll, 700)
            else polling = false
        } catch (error) {
            for (const view of views.values()) view.summary.textContent = `Transfers · ${error.message} · retrying…`
            if (views.size) setTimeout(poll, 2000); else polling = false
        }
    }
    async function open(category, index) {
        const host = hostsData[Number(category) - 1]?.hosts[Number(index) - 1]
        if (!host) return
        const id = `sftp-${host['unique-id']}`
        return openConnection(id, host.name, { hostsFile, hostId: host['unique-id'] }, 'SFTP')
    }
    async function openConnection(id, title, request, protocol) {
        if (views.has(id)) { workspaceTabs.select(id); return }
        const content = element('section', undefined, 'file-browser')
        const view = { protocol, content, controller: new AbortController(), closed: false, status: element('div', undefined, 'file-status') }
        view.status.setAttribute('role', 'status')
        view.local = pane(view, 'local'); view.remote = pane(view, 'remote')
        const panes = element('div', undefined, 'file-panes')
        panes.append(view.local.node, view.remote.node)
        view.status.hidden = true
        content.append(panes, view.status, createQueue(view))
        views.set(id, view)
        workspaceTabs.open({ id, title: `${title} · ${protocol}`, content, onClose: async () => {
            // This preference lasts only for this app page; it is never persisted.
            if (view.session && !skipCloseConfirmation && !await appDialogs.confirm('Any queued or running transfers for this connection will be cancelled.', {
                title: 'Close connection tab', confirmText: 'Close tab',
                checkboxLabel: 'Do not ask again during this session',
                onCheckboxConfirm: checked => { skipCloseConfirmation = checked },
            })) return false
            try { if (view.session) await api(`/files/sessions/${view.session}`, 'DELETE') }
            catch (error) { status(view, error.message, true); return false }
            view.closed = true; view.controller.abort(); views.delete(id)
        } })
        status(view, `Connecting to ${protocol}…`)
        try {
            // Do not abort session creation: if the tab closes while connecting,
            // consume the ID and explicitly close it so credentials are not orphaned.
            const session = await api('/files/sessions', 'POST', request)
            if (request.connection) request.connection.password = ''
            if (view.closed) { await api(`/files/sessions/${session.id}`, 'DELETE'); return }
            view.session = session.id
            status(view, 'Select files or folders, then Upload or Download. Links are not transferred.')
            view.local.home = session.local; view.remote.home = session.remote
            await Promise.all([load(view, 'local', session.local), load(view, 'remote', session.remote)])
        } catch (error) { if (!view.closed) status(view, `${error.message}. Close this tab and retry.`, true) }
        finally { if (request.connection) request.connection.password = '' }
    }
    function quickConnect() {
        if (document.querySelector('#file-connect-dialog')) return
        const dialog = element('dialog'); dialog.id = 'file-connect-dialog'
        const form = element('form')
        const protocol = element('select'); protocol.name = 'protocol'
        for (const [value, text] of [['ftps', 'FTPS · explicit TLS'], ['ftps-implicit', 'FTPS · implicit TLS'], ['ftp', 'FTP · unencrypted']]) {
            const option = element('option', text); option.value = value; protocol.append(option)
        }
        const fields = { protocol }
        const title = element('h2', 'New file connection'); title.id = 'file-connect-title'
        dialog.setAttribute('aria-labelledby', title.id)
        dialog.append(title)
        for (const [name, title, type, value] of [
            ['protocol', 'Protocol'], ['address', 'Server', 'text', ''], ['port', 'Port', 'number', '21'],
            ['username', 'Username', 'text', ''], ['password', 'Password', 'password', ''],
        ]) {
            const input = fields[name] || element('input'); input.name = name
            if (name !== 'protocol') { input.type = type; input.value = value; input.autocomplete = 'off' }
            if (name === 'address' || name === 'port') input.required = true
            if (name === 'port') { input.min = 1; input.max = 65535 }
            fields[name] = input
            const label = element('label', title)
            if (name === 'password') {
                input.id = 'file-connect-password'
                const field = element('span', undefined, 'modal-password-field')
                const toggle = button('', () => togglePasswordVisibility(toggle))
                toggle.className = 'password-toggle'; toggle.dataset.passwordToggle = input.id
                toggle.title = 'Show password'; toggle.setAttribute('aria-label', 'Show password'); toggle.setAttribute('aria-pressed', 'false')
                const icon = element('span', 'visibility', 'material-symbols-outlined'); icon.setAttribute('aria-hidden', 'true'); toggle.append(icon)
                field.append(input, toggle); label.append(field)
            } else label.append(input)
            form.append(label)
        }
        const note = element('p', 'Credentials are held only in this tab, not saved. FTPS verifies the server certificate.')
        protocol.addEventListener('change', () => {
            fields.port.value = protocol.value === 'ftps-implicit' ? '990' : '21'
            note.textContent = protocol.value === 'ftp' ? 'FTP sends credentials and files unencrypted. Settings are not saved.' : 'Credentials are held only in this tab, not saved. FTPS verifies the server certificate.'
        })
        const buttons = element('div', undefined, 'file-actions')
        const submit = element('button', 'Connect', 'ok'); submit.type = 'submit'
        submit.title = 'Connect (Enter in input field)'
        const cancel = button('Cancel', () => dialog.close()); cancel.title = 'Cancel (Esc)'
        buttons.append(cancel, submit)
        form.append(note, buttons); dialog.append(form); document.body.append(dialog)
        dialog.addEventListener('close', () => { fields.password.value = ''; dialog.remove() }, { once: true })
        form.addEventListener('submit', async event => {
            event.preventDefault()
            const connection = Object.fromEntries(new FormData(form)); connection.port = Number(connection.port)
            if (submit.disabled) return
            submit.disabled = true
            if (connection.protocol === 'ftp' && !await appDialogs.confirm('FTP sends credentials and files without encryption. Connect anyway?', { title: 'Unencrypted FTP connection', confirmText: 'Connect' })) { submit.disabled = false; return }
            dialog.close()
            openConnection(`ftp-${crypto.randomUUID()}`, connection.address, { connection }, connection.protocol.toUpperCase())
        })
        dialog.showModal(); fields.address.focus()
    }
    return { open, quickConnect }
})()

function openFileBrowser(category, index) { fileBrowser.open(category, index) }
