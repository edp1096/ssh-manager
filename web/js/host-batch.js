const hostBatch = (() => {
    const selected = new Set()
    let busy = false
    let groupData = [], activeGroup = '', changing = false, groupLoaded = false, groupMenu = null
    let expandedGroup = '', draggedHost = null, draggedGroup = null, picker = null, groupRevision = 0
    let panelPinned = false, panelSettingsFile = null
    const container = document.querySelector('#hosts-data-container')
    function hosts() {
        return hostsData.flatMap(category => category.hosts || [])
    }
    function sync() {
        if (panelSettingsFile !== hostsFile) restorePanelSettings()
        const available = new Set(hosts().map(h => h['unique-id']).filter(Boolean))
        for (const id of selected) if (!available.has(id)) selected.delete(id)
        for (const row of container.querySelectorAll('.host-part-info')) {
            const input = row.querySelector('.host-select')
            if (!input) continue
            input.checked = selected.has(row.dataset.hostId)
            input.disabled = !row.dataset.hostId || busy
            input.setAttribute('aria-label', `Select ${row.getAttribute('aria-label') || 'host'}`)
            row.classList.toggle('host-checked', input.checked)
        }
        document.querySelector('#host-selection-count').textContent = `${selected.size} selected`
        document.querySelector('#host-batch-open').disabled = busy || !selected.size
        document.querySelector('#host-batch-save').disabled = busy || !selected.size
        document.querySelector('#host-batch-delete').disabled = busy || !selected.size
        if (groupLoaded) renderGroups()
    }
    function toggle(id) { if (!id || busy) return; if (selected.has(id)) selected.delete(id); else selected.add(id); sync() }
    async function deleteSelected() {
        if (busy || !selected.size) return
        const chosen = ids(), file = hostsFile
        const names = hosts().filter(h => chosen.includes(h['unique-id'])).map(h => h.name)
        busy = true; sync()
        try {
            if (!await appDialogs.confirm(`Delete ${chosen.length} selected hosts from the saved host list? This cannot be undone.\n\n${names.join('\n')}`, {title: 'Delete selected hosts', confirmText: 'Delete', danger: true})) return
            const response = await fetch('/hosts/selected?' + new URLSearchParams({'hosts-file': file}), {
                method: 'DELETE', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({'host-ids': chosen}),
            })
            if (!response.ok) throw new Error(await response.text())
            for (const id of chosen) selected.delete(id)
            await getHosts()
        } catch (error) { await appDialogs.alert(error.message, {title: 'Could not delete hosts'}) }
        finally { busy = false; sync() }
    }
    container.addEventListener('change', event => { if (event.target.matches('.host-select')) toggle(event.target.closest('.host-part-info').dataset.hostId) })
    function ids() { return hosts().filter(h => selected.has(h['unique-id'])).map(h => h['unique-id']) }
    function layoutFields(value) { return value.startsWith('grid:') ? { layout: 'grid', columns: Number(value.split(':')[1]), 'grid-fill': value.split(':')[2] || 'horizontal' } : { layout: value, columns: 0 } }
    function addGridOptions(select) { for (let columns = 1; columns <= 32; columns++) { for (const fill of ['horizontal', 'vertical', 'vertical-left']) { const option=document.createElement('option'); option.value=`grid:${columns}${fill === 'horizontal' ? '' : ':' + fill}`; option.textContent=`Grid · ${columns} columns · ${fill}`; select.append(option) } } }
    function gridLabel(columns, rows) { return `${columns} ${columns === 1 ? 'column' : 'columns'} × ${rows} ${rows === 1 ? 'row' : 'rows'}` }
    function closePicker(focus = false) { if (!picker) return; const { popup, anchor } = picker; picker = null; popup.remove(); anchor.setAttribute('aria-expanded', 'false'); if (focus && anchor.isConnected) anchor.focus() }
    document.addEventListener('pointerdown', event => { if (picker && !picker.popup.contains(event.target) && !picker.anchor.contains(event.target)) closePicker() })
    window.addEventListener('resize', () => closePicker())
    function attachLayoutPicker(select, hostIDs, saveFill) {
        select.hidden = true
        const button = document.createElement('button'); button.type = 'button'; button.className = 'grid-picker-toggle'; button.dataset.action = 'layout-picker'
        button.setAttribute('aria-haspopup', 'dialog'); button.setAttribute('aria-expanded', 'false')
        const update = () => { const fields = layoutFields(select.value); button.textContent = fields.layout === 'grid' ? gridLabel(fields.columns, Math.ceil(hostIDs.length / fields.columns)) + (fields['grid-fill'].startsWith('vertical') ? ' · ↕' : '') : select.selectedOptions[0]?.textContent; button.title = fields.layout === 'grid' ? `Expand ${fields['grid-fill'].startsWith('vertical') ? 'vertically' : 'horizontally'}` : button.textContent; button.disabled = select.disabled }
        select.after(button); update(); select.addEventListener('change', update)
        button.onclick = () => {
            if (picker?.anchor === button) { closePicker(); return }
            closePicker(); const popup = document.createElement('div'); popup.className = 'layout-picker'; popup.setAttribute('role', 'dialog'); popup.setAttribute('aria-label', 'Split layout')
            const choose = value => { closePicker(true); select.value = value; select.dispatchEvent(new Event('change')); update() }
            let fill = layoutFields(select.value)['grid-fill'] || 'horizontal'
            const gridValue = c => `grid:${c}${fill === 'horizontal' ? '' : ':' + fill}`
            for (const [value, text] of [['alternating', 'Alternating splits'], ['horizontal', 'Left / right'], ['vertical', 'Top / bottom']]) {
                const option = document.createElement('button'); option.type = 'button'; option.textContent = text; option.onclick = () => choose(value); popup.append(option)
            }
            const caption = document.createElement('p'); caption.className = 'grid-caption'; caption.setAttribute('aria-live', 'polite')
            const grid = document.createElement('div'); grid.className = 'layout-grid'; grid.setAttribute('role', 'grid'); grid.setAttribute('aria-label', 'Grid size: columns by rows')
            const columns = Math.max(4, hostIDs.length, layoutFields(select.value).columns || 0), rows = Math.max(4, hostIDs.length), cells = [], currentNames = new Map(hosts().map(h => [h['unique-id'], h.name]))
            grid.style.setProperty('--grid-columns', columns)
            const viewport = document.createElement('div'); viewport.className = 'layout-grid-viewport'; viewport.append(grid)
            let column = select.value.startsWith('grid:') ? layoutFields(select.value).columns : Math.min(3, Math.max(2, hostIDs.length)), row = Math.ceil(hostIDs.length / column)
            const valid = (c, r) => c >= 1 && r === Math.ceil(hostIDs.length / c)
            const arrangement = document.createElement('div'); arrangement.className = 'grid-arrangement'; arrangement.setAttribute('aria-label', 'Panel arrangement preview')
            const gridIndex = (c, y, x) => {
                const gap = fill === 'vertical-left' && hostIDs.length > c && hostIDs.length % c ? c - hostIDs.length % c : 0
                if (gap && y === Math.floor(hostIDs.length / c)) return x < gap ? -1 : y * c + x - gap
                return y * c + x
            }
            const preview = (c, r) => {
                column = c; row = r
                arrangement.replaceChildren(); arrangement.style.visibility = valid(c,r) ? 'visible' : 'hidden'
                arrangement.style.flexDirection = fill.startsWith('vertical') ? 'row' : 'column'
                const major = fill.startsWith('vertical') ? Math.min(c, hostIDs.length) : Math.ceil(hostIDs.length/c)
                for (let a=0; a<major; a++) {
                    const strip=document.createElement('div'); strip.style.flexDirection=fill.startsWith('vertical') ? 'column' : 'row'
                    const minor=fill.startsWith('vertical') ? Math.ceil(hostIDs.length/c) : Math.min(c,hostIDs.length-a*c)
                    for(let b=0;b<minor;b++) {
                        const index=fill.startsWith('vertical') ? gridIndex(c,b,a) : a*c+b
                        if (index < 0 || index >= hostIDs.length) continue
                        const cell=document.createElement('span');cell.textContent=String(index+1);cell.title=currentNames.get(hostIDs[index]) || '[Missing host]';strip.append(cell)
                    }
                    arrangement.append(strip)
                }
                caption.textContent = `${gridLabel(c,r)}${valid(c,r) ? '' : ' — unavailable'}`
                for (const cell of cells) {
                    const x = Number(cell.dataset.column), y = Number(cell.dataset.row), inside = x <= c && y <= r, index = gridIndex(c, y - 1, x - 1)
                    cell.classList.toggle('in-range', inside); cell.classList.toggle('unused', inside && (index < 0 || index >= hostIDs.length))
                    cell.textContent = inside && index >= 0 && index < hostIDs.length ? String(index + 1) : ''
                    cell.title = inside && index >= 0 && index < hostIDs.length ? `${index + 1}. ${currentNames.get(hostIDs[index]) || '[Missing host]'}` : gridLabel(x,y)
                    cell.tabIndex = x === c && y === r ? 0 : -1
                }
            }
            for (let y = 1; y <= rows; y++) {
                const gridRow = document.createElement('div'); gridRow.setAttribute('role', 'row')
                for (let x = 1; x <= columns; x++) {
                    const cell = document.createElement('button'); cell.type = 'button'; cell.setAttribute('role', 'gridcell'); cell.dataset.column = x; cell.dataset.row = y
                    cell.setAttribute('aria-label', gridLabel(x,y)); cell.setAttribute('aria-disabled', String(!valid(x,y)))
                    cell.onpointerenter = () => preview(x,y)
                    cell.onclick = () => { if (valid(x,y)) choose(gridValue(x)) }
                    gridRow.append(cell); cells.push(cell)
                }
                grid.append(gridRow)
            }
            grid.addEventListener('keydown', event => {
                if (event.isComposing) return
                if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault(); event.stopPropagation()
                    const c = Number(event.target.dataset.column), r = Number(event.target.dataset.row)
                    if (valid(c,r)) choose(gridValue(c))
                    return
                }
                const delta = {ArrowLeft:[-1,0],ArrowRight:[1,0],ArrowUp:[0,-1],ArrowDown:[0,1]}[event.key]
                if (!delta) return; event.preventDefault(); event.stopPropagation()
                const focusedColumn = Number(event.target.dataset.column), focusedRow = Number(event.target.dataset.row)
                preview(Math.max(1,Math.min(columns,focusedColumn+delta[0])),Math.max(1,Math.min(rows,focusedRow+delta[1])))
                cells.find(cell => Number(cell.dataset.column) === column && Number(cell.dataset.row) === row)?.focus()
            })
            const fillLabel = document.createElement('label'); fillLabel.className = 'grid-fill-label'; fillLabel.textContent = 'Expand'
            const fillSelect = document.createElement('select'); fillSelect.className = 'grid-fill-select'; fillSelect.setAttribute('aria-label', 'Expand incomplete grid')
            for (const [value, text] of [['horizontal','Horizontally'],['vertical','Vertically'],['vertical-left','Vertically (left)']]) { const option = document.createElement('option'); option.value=value; option.textContent=text; fillSelect.append(option) }
            fillSelect.value=fill
            fillSelect.onchange=async()=>{
                const previousFill = fill
                fill=fillSelect.value
                const selectedColumns = layoutFields(select.value).columns || column
                const value = gridValue(selectedColumns)
                const controls = [...popup.querySelectorAll('button, select')]
                controls.forEach(control => { control.disabled = true })
                try {
                    if (saveFill) await saveFill(layoutFields(value))
                    select.value=value; update()
                    preview(selectedColumns, Math.ceil(hostIDs.length/selectedColumns)); updateNote()
                } catch (error) {
                    fill=previousFill; fillSelect.value=fill
                    await appDialogs.alert(error.message, {title:'Could not save grid layout'})
                } finally { controls.forEach(control => { control.disabled = false }) }
            }
            fillLabel.append(fillSelect)
            const note = document.createElement('p'); note.className = 'grid-note'
            const updateNote = () => { note.textContent = `${hostIDs.length} hosts · ${fill.startsWith('vertical') ? 'Each column shares its height among its hosts. Shorter columns expand vertically.' : 'Each row shares its width among its hosts. The last row expands horizontally.'}` }
            updateNote()
            popup.append(caption,viewport,fillLabel,arrangement,note); preview(column,row)
            popup.addEventListener('keydown', event => { if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); closePicker(true) } })
            popup.addEventListener('focusout', event => {
                // focusout runs before the browser completes the next focus.
                // Reading activeElement in a microtask can see body and remove
                // the clicked cell before its click event is dispatched.
                if (event.relatedTarget && !popup.contains(event.relatedTarget) && event.relatedTarget !== button) closePicker()
            })
            ;(button.closest('dialog') || document.body).append(popup); picker = { popup, anchor: button }; button.setAttribute('aria-expanded', 'true')
            const rect = button.getBoundingClientRect(), size = popup.getBoundingClientRect()
            popup.style.left = `${Math.max(8, Math.min(rect.left, innerWidth-size.width-8))}px`; popup.style.top = `${Math.max(8,Math.min(rect.bottom+4,innerHeight-size.height-8))}px`
            cells.find(cell => cell.tabIndex === 0)?.focus()
        }
        return button
    }
    async function connectGroup(group) {
        if (busy || changing) return
        busy = true; sync()
        result.textContent = `${group.name}: Opening panels…`
        let message
        try {
            const response = await fetch('/session/batch', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ 'hosts-file': hostsFile, 'host-ids': group['host-ids'], layout: group.layout || 'alternating', columns: group.columns || 0, 'grid-fill': group['grid-fill'] || 'horizontal' }) })
            if (!response.ok) throw new Error(await response.text())
            const data = await response.json()
            message = data.error || `${data.opened} launch request(s) completed. Check authentication in each terminal.`
        } catch (error) { message = `${error.message} Check opened terminals before retrying.` }
        finally { busy = false; sync(); result.textContent = `${group.name}: ${message}` }
    }
    async function open(hostIDs = ids(), initialLayout = 'alternating') {
        if (busy || !hostIDs.length) return
        if (hostIDs.length > 32) { await appDialogs.alert('Select no more than 32 hosts per window.'); return }
        const names = new Map(hosts().map(h => [h['unique-id'], h.name]))
        if (hostIDs.some(id => !names.has(id))) { await appDialogs.alert('A saved host no longer exists. Update the group before connecting.'); return }
        const dialog = document.createElement('dialog'); dialog.id = 'host-batch-dialog'
        dialog.setAttribute('aria-labelledby', 'host-batch-title')
        dialog.innerHTML = `<h2 id="host-batch-title">Open in panels</h2><p>A new terminal window will open. Each following host is added as a split panel. Broadcast input stays off for these new connections.</p><ol></ol><label class="batch-layout">Layout<select><option value="alternating">Alternating splits</option><option value="horizontal">Left / right</option><option value="vertical">Top / bottom</option></select></label><p class="batch-result" role="status"></p><footer class="modal-actions"><button type="button" data-close title="Close (Esc)">Cancel</button><button type="button" data-connect class="ok">Connect</button></footer>`
        dialog.querySelector('p').textContent = 'Open these hosts in a new terminal window using the selected layout. Grid fills left to right, then top to bottom. Broadcast input stays off.'
        dialog.querySelector('.batch-layout').firstChild.textContent = 'Split'
        for (const id of hostIDs) { const li = document.createElement('li'); li.textContent = names.get(id); dialog.querySelector('ol').append(li) }
        addGridOptions(dialog.querySelector('select')); dialog.querySelector('select').value = initialLayout
        const layoutButton = attachLayoutPicker(dialog.querySelector('select'), hostIDs)
        const close = dialog.querySelector('[data-close]'), connect = dialog.querySelector('[data-connect]'), result = dialog.querySelector('.batch-result')
        close.onclick = () => dialog.close()
        dialog.addEventListener('cancel', event => {
            if (picker?.anchor === layoutButton) { event.preventDefault(); closePicker(true) }
            else if (busy) event.preventDefault()
        })
        dialog.addEventListener('close', () => { if (picker?.anchor === layoutButton) closePicker(); dialog.remove() }, { once: true })
        connect.onclick = async () => {
            busy = true; sync(); connect.disabled = true; close.disabled = true; dialog.querySelector('select').disabled = true
            closePicker(); layoutButton.disabled = true
            dialog.tabIndex = -1; dialog.focus()
            result.textContent = 'Opening panels in order…'
            try {
                const response = await fetch('/session/batch', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ 'hosts-file': hostsFile, 'host-ids': hostIDs, ...layoutFields(dialog.querySelector('select').value) }) })
                if (!response.ok) throw new Error(await response.text())
                const data = await response.json()
                result.textContent = data.error || `${data.opened} launch request(s) completed. Check authentication in each terminal.`
            } catch (error) { result.textContent = `${error.message} Check opened terminals before retrying.` }
            finally { busy = false; sync(); close.disabled = false; close.textContent = 'Close'; close.focus() }
        }
        document.body.append(dialog); dialog.showModal(); close.focus()
    }
    async function groupRequest(method = 'GET', group, settings = false) {
        const response = await fetch('/connection-groups?' + new URLSearchParams({ 'hosts-file': hostsFile, ...(settings ? {settings: 'panel'} : {}) }), {
            method, headers: { 'Content-Type': 'application/json' }, body: group ? JSON.stringify(group) : undefined,
        })
        if (!response.ok) throw new Error(await response.text())
        return response.json()
    }
    async function saveGroup() {
        const chosen = ids()
        if (busy || !chosen.length) return
        if (chosen.length > 32) { await appDialogs.alert('Select no more than 32 hosts per group.'); return }
        const name = await appDialogs.prompt(`Save ${chosen.length} selected hosts as a connection group.`, { title: 'Save connection group', label: 'Name', confirmText: 'Save' })
        if (!name) return
        try { groupData = await groupRequest('POST', { name, 'host-ids': chosen, layout: 'alternating' }); activeGroup = groupData.at(-1)?.id || ''; await groups() }
        catch (error) { await appDialogs.alert(error.message, { title: 'Connection groups' }) }
    }
    const panel = document.createElement('aside'); panel.id = 'connection-groups-panel'; panel.hidden = true
    panel.setAttribute('aria-labelledby', 'connection-groups-title')
    panel.innerHTML = `<header><h2 id="connection-groups-title">Connection groups</h2><button type="button" data-pin aria-pressed="false" title="Pin panel" aria-label="Pin panel"><span class="material-symbols-outlined" aria-hidden="true">push_pin</span></button><button type="button" data-close title="Hide groups (Esc)" aria-label="Hide connection groups">×</button></header><div class="connection-group-list"></div><p class="group-result" role="status"></p>`
    container.append(panel)
    const list = panel.querySelector('.connection-group-list'), result = panel.querySelector('.group-result')
    const groupToggle = container.querySelector('[onclick="hostBatch.groups()"]')
    groupToggle.setAttribute('aria-controls', panel.id); groupToggle.setAttribute('aria-expanded', 'false')
    const pinButton = panel.querySelector('[data-pin]')
    const sideButton = document.createElement('button'); sideButton.type = 'button'; sideButton.dataset.side = ''
    sideButton.innerHTML = '<span class="material-symbols-outlined" aria-hidden="true">right_panel_open</span>'
    pinButton.before(sideButton)
    let panelSide = 'left'
    function updateSide() {
        closeGroupMenu()
        container.classList.toggle('groups-right', panelSide === 'right')
        sideButton.title = panelSide === 'right' ? 'Move panel to left' : 'Move panel to right'
        sideButton.setAttribute('aria-label', sideButton.title)
        sideButton.querySelector('span').textContent = panelSide === 'right' ? 'left_panel_open' : 'right_panel_open'
    }
    updateSide(); sideButton.disabled = true
    fetch('/window-size', {cache:'no-store'}).then(async response => {
        if (!response.ok) throw new Error('Could not load panel position')
        const settings = await response.json()
        panelSide = settings['panel-side'] === 'right' ? 'right' : 'left'; updateSide()
    }).catch(error => { result.textContent = error.message }).finally(() => { sideButton.disabled = false })
    sideButton.onclick = async () => {
        sideButton.disabled = true
        const side = panelSide === 'left' ? 'right' : 'left'
        try {
            const response = await fetch('/window-size', {method:'PATCH', headers:{'Content-Type':'application/json'},body:JSON.stringify({'panel-side':side})})
            if (!response.ok) throw new Error(await response.text())
            panelSide = side; updateSide()
        } catch (error) { await appDialogs.alert(error.message, {title:'Panel position'}) }
        finally { sideButton.disabled = false }
    }
    function updatePin() {
        pinButton.setAttribute('aria-pressed', String(panelPinned))
        pinButton.title = panelPinned ? 'Unpin panel' : 'Pin panel'
        pinButton.setAttribute('aria-label', pinButton.title)
        panel.querySelector('[data-close]').disabled = panelPinned
        groupToggle.disabled = panelPinned
    }
    async function restorePanelSettings() {
        const file = hostsFile; panelSettingsFile = file; pinButton.disabled = true
        try {
            const settings = await groupRequest('GET', undefined, true)
            if (hostsFile !== file) return
            panelPinned = settings['panel-pinned'] === true; updatePin()
            if (panelPinned) await groups()
        } catch (error) { result.textContent = error.message }
        finally { if (hostsFile === file) pinButton.disabled = false }
    }
    pinButton.onclick = async () => {
        pinButton.disabled = true
        try {
            const settings = await groupRequest('PUT', {'panel-pinned': !panelPinned}, true)
            panelPinned = settings['panel-pinned'] === true; updatePin()
        } catch (error) { await appDialogs.alert(error.message, {title: 'Panel settings'}) }
        finally { pinButton.disabled = false }
    }
    function hideGroups() { if (panelPinned) return; closeGroupMenu(); panel.hidden = true; container.classList.remove('groups-open'); groupToggle.setAttribute('aria-expanded', 'false'); groupToggle.focus() }
    panel.querySelector('[data-close]').onclick = hideGroups
    panel.addEventListener('keydown', event => {
        if (event.key === 'Escape' && !event.isComposing) { event.preventDefault(); event.stopPropagation(); hideGroups() }
        if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key) && event.target.matches('.connection-group-name')) {
            const buttons = [...list.querySelectorAll('.connection-group-name')], i = buttons.indexOf(event.target)
            const next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length - 1 : Math.max(0, Math.min(buttons.length - 1, i + (event.key === 'ArrowDown' ? 1 : -1)))
            event.preventDefault(); event.stopPropagation(); buttons[next]?.focus()
        }
    })
    function closeGroupMenu() { groupMenu?.remove(); groupMenu = null; closePicker() }
    document.addEventListener('pointerdown', event => { if (groupMenu && !groupMenu.contains(event.target)) closeGroupMenu() })
    document.addEventListener('scroll', event => { if (!picker?.popup.contains(event.target)) closeGroupMenu() }, true)
    window.addEventListener('resize', closeGroupMenu)
    new MutationObserver(() => { if (container.hidden) closeGroupMenu() }).observe(container, { attributes: true, attributeFilter: ['hidden'] })
    async function changeGroup(action, focusAction, focusGroup) {
        if (changing || busy) return
        groupRevision++
        changing = true; closeGroupMenu(); renderGroups()
        let failure = ''
        try { const data = await action(); if (data) groupData = data }
        catch (error) { failure = error.message }
        finally { changing = false; renderGroups(); if (failure) result.textContent = failure; if (focusAction) [...list.querySelectorAll('[data-action]')].find(node => node.dataset.action === focusAction && (!focusGroup || node.closest('[data-group-id]')?.dataset.groupId === focusGroup))?.focus({preventScroll:true}) }
    }
    function reorderGroup(id, to) {
        const order = groupData.map(group => group.id), from = order.indexOf(id)
        if (from < 0 || from === to || to < 0 || to >= order.length) return
        order.splice(to, 0, order.splice(from, 1)[0])
        return groupRequest('PATCH', {'group-ids': order})
    }
    function moveGroup(id, to) {
        if (changing || busy) return
        return changeGroup(() => reorderGroup(id, to), 'select', id)
    }
    function showGroupMenu(group, anchor, x, y) {
        closeGroupMenu()
        const menu = document.createElement('div'); menu.className = 'file-context-menu connection-group-menu'; menu.setAttribute('role', 'menu')
        const add = (label, action, disabled = false) => {
            const button = document.createElement('button'); button.type = 'button'; button.textContent = label; button.disabled = disabled || changing || busy; button.setAttribute('role', 'menuitem')
            button.onclick = () => { closeGroupMenu(); anchor.focus(); changeGroup(action, 'menu', group.id) }; menu.append(button)
        }
        const index = groupData.findIndex(item => item.id === group.id)
        add('Move up', () => reorderGroup(group.id, index - 1), index <= 0)
        add('Move down', () => reorderGroup(group.id, index + 1), index >= groupData.length - 1)
        add('Rename', async () => { const name = await appDialogs.prompt('Connection group name', { title: 'Rename connection group', label: 'Name', value: group.name, confirmText: 'Rename' }); if (name) return groupRequest('PUT', { ...group, name }) })
        add('Replace hosts', async () => { const chosen = ids(), existing = new Set(group['host-ids']), kept = group['host-ids'].filter(id => chosen.includes(id)), ordered = [...kept, ...chosen.filter(id => !existing.has(id))]; if (await appDialogs.confirm(`Replace “${group.name}” with the ${chosen.length} currently selected hosts? Existing order is preserved; new hosts are appended.`, { title: 'Update connection group', confirmText: 'Replace' })) return groupRequest('PUT', { ...group, 'host-ids': ordered }) }, !selected.size || selected.size > 32)
        add('Delete', async () => { if (await appDialogs.confirm(`Delete connection group “${group.name}”? Saved hosts will not be deleted.`, { title: 'Delete connection group', confirmText: 'Delete', danger: true })) return groupRequest('DELETE', { id: group.id }) })
        document.body.append(menu); groupMenu = menu
        const rect = menu.getBoundingClientRect(); menu.style.left = `${Math.max(0, Math.min(x, innerWidth - rect.width))}px`; menu.style.top = `${Math.max(0, Math.min(y, innerHeight - rect.height))}px`
        menu.addEventListener('keydown', event => {
            if (event.key === 'Escape' || event.key === 'Tab') { event.preventDefault(); event.stopPropagation(); closeGroupMenu(); anchor.focus(); return }
            const buttons = [...menu.querySelectorAll('button:not(:disabled)')], i = buttons.indexOf(document.activeElement)
            if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); event.stopPropagation(); buttons[(i + (event.key === 'ArrowDown' ? 1 : buttons.length - 1)) % buttons.length]?.focus() }
        })
        menu.querySelector('button:not(:disabled)')?.focus({ preventScroll: true })
    }
    function renderGroups() {
        closeGroupMenu()
        const previous = document.activeElement, focusID = previous.closest?.('[data-group-id]')?.dataset.groupId, focusAction = previous.dataset?.action
        const current = new Map(hosts().map(h => [h['unique-id'], h.name]))
        list.replaceChildren()
        result.textContent = groupData.length ? '' : 'Select hosts, then Save selection.'
        for (const group of groupData) {
            const card = document.createElement('section'); card.dataset.groupId = group.id; card.classList.toggle('active', activeGroup === group.id)
            const heading = document.createElement('button'); heading.type = 'button'; heading.className = 'connection-group-name'; heading.dataset.action = 'select'; heading.textContent = group.name; heading.title = group.name; heading.disabled = busy || changing
            heading.setAttribute('aria-pressed', String(activeGroup === group.id))
            heading.setAttribute('aria-expanded', String(expandedGroup === group.id))
            heading.draggable = !heading.disabled
            heading.title = `${group.name} - Drag to reorder (Alt+Up/Down)`
            heading.onkeydown = event => {
                if (!event.altKey || !['ArrowUp','ArrowDown'].includes(event.key) || event.isComposing) return
                event.preventDefault(); event.stopPropagation()
                const index = groupData.findIndex(item => item.id === group.id)
                moveGroup(group.id, index + (event.key === 'ArrowUp' ? -1 : 1))
            }
            heading.ondragstart = event => {
                if (busy || changing) { event.preventDefault(); return }
                closeGroupMenu(); draggedGroup = group.id
                event.dataTransfer.effectAllowed = 'move'; event.dataTransfer.setData('text/plain', group.id)
            }
            heading.ondragend = () => {
                draggedGroup = null
                for (const item of list.children) item.classList.remove('group-drop-before', 'group-drop-after')
            }
            card.ondragover = event => {
                if (!draggedGroup || draggedGroup === group.id || busy || changing) return
                event.preventDefault(); event.dataTransfer.dropEffect = 'move'
                const after = event.clientY > card.getBoundingClientRect().top + card.offsetHeight / 2
                card.classList.toggle('group-drop-after', after); card.classList.toggle('group-drop-before', !after)
            }
            card.ondragleave = event => {
                if (!card.contains(event.relatedTarget)) card.classList.remove('group-drop-before', 'group-drop-after')
            }
            card.ondrop = event => {
                if (!draggedGroup || busy || changing) return
                event.preventDefault(); event.stopPropagation()
                const id = draggedGroup, from = groupData.findIndex(item => item.id === id), target = groupData.findIndex(item => item.id === group.id)
                const after = event.clientY > card.getBoundingClientRect().top + card.offsetHeight / 2
                heading.ondragend()
                if (id === group.id || from < 0) return
                const insertion = target + (after ? 1 : 0)
                moveGroup(id, insertion - (from < insertion ? 1 : 0))
            }
            heading.onclick = () => {
                expandedGroup = expandedGroup === group.id ? '' : group.id
                activeGroup = group.id; selected.clear(); for (const id of group['host-ids']) if (current.has(id)) selected.add(id); sync()
                for (const category of container.querySelectorAll('.category')) if (category.querySelector('.host-checked')) setCategoryExpanded(category, true)
            }
            const missing = group['host-ids'].filter(id => !current.has(id))
            const names = document.createElement('p'); names.className = missing.length ? 'group-missing' : 'group-members'; names.textContent = `${group['host-ids'].length} hosts${missing.length ? ` · ${missing.length} missing` : ''}`; names.title = group['host-ids'].map(id => current.get(id) || '[Missing host]').join('\n')
            const label = document.createElement('label'); label.textContent = 'Split'
            const layout = document.createElement('select'); layout.dataset.action = 'layout'; layout.setAttribute('aria-label', `Split: ${group.name}`)
            for (const [value, text] of [['alternating','Alternating splits'],['horizontal','Left / right'],['vertical','Top / bottom']]) { const option = document.createElement('option'); option.value = value; option.textContent = text; layout.append(option) }
            addGridOptions(layout); layout.value = group.layout === 'grid' ? `grid:${group.columns}${group['grid-fill']?.startsWith('vertical') ? ':' + group['grid-fill'] : ''}` : group.layout || 'alternating'; layout.disabled = changing || busy
            layout.onchange = () => { const fields = layoutFields(layout.value); changeGroup(() => groupRequest('PUT', { ...group, ...fields })) }; label.append(layout)
            attachLayoutPicker(layout, group['host-ids'], async fields => {
                if (changing || busy) throw new Error('Please wait for the current operation.')
                changing = true; groupRevision++
                try {
                    const data = await groupRequest('PUT', {...group, ...fields})
                    Object.assign(group, data.find(item => item.id === group.id))
                    groupData = data
                } finally { changing = false }
            })
            const actions = document.createElement('div'); actions.className = 'connection-group-actions'
            const connect = document.createElement('button'); connect.type = 'button'; connect.textContent = 'Connect'; connect.dataset.action = 'connect'; connect.disabled = !!missing.length || busy || changing; connect.onclick = () => connectGroup(group)
            const more = document.createElement('button'); more.type = 'button'; more.textContent = '⋯'; more.dataset.action = 'menu'; more.title = 'Group actions'; more.setAttribute('aria-label', `Actions: ${group.name}`); more.setAttribute('aria-haspopup', 'menu'); more.disabled = changing || busy
            more.onclick = () => { const rect = more.getBoundingClientRect(); showGroupMenu(group, more, rect.left, rect.bottom) }
            card.oncontextmenu = event => { event.preventDefault(); event.stopPropagation(); if (!changing && !busy) showGroupMenu(group, more, event.clientX, event.clientY) }
            const titleRow = document.createElement('div'); titleRow.className = 'connection-group-heading'; titleRow.append(heading, names)
            actions.append(connect, more); card.append(titleRow, label, actions); list.append(card)
            if (expandedGroup === group.id) {
                const members = document.createElement('ol'); members.className = 'connection-group-order'; members.setAttribute('aria-label', `Host order: ${group.name}`)
                const move = (from, to) => {
                    if (busy || changing || from === to || to < 0 || to >= group['host-ids'].length) return
                    const order = [...group['host-ids']], [id] = order.splice(from, 1); order.splice(to, 0, id)
                    changeGroup(() => groupRequest('PUT', { ...group, 'host-ids': order }), `member:${group.id}:${id}`)
                }
                group['host-ids'].forEach((id, index) => {
                    const li = document.createElement('li'), member = document.createElement('button'); member.type = 'button'; member.dataset.action = `member:${group.id}:${id}`; member.dataset.hostId = id
                    member.textContent = `${index + 1}. ${current.get(id) || '[Missing host]'}`; member.title = `${current.get(id) || '[Missing host]'} — Drag to reorder (Alt+↑/↓)`; member.disabled = busy || changing; member.draggable = !member.disabled
                    member.onkeydown = event => {
                        if (!['ArrowUp','ArrowDown'].includes(event.key) || event.isComposing) return
                        event.preventDefault(); event.stopPropagation(); const next = index + (event.key === 'ArrowUp' ? -1 : 1)
                        if (event.altKey) move(index,next); else members.children[Math.max(0,Math.min(members.children.length-1,next))]?.querySelector('button').focus()
                    }
                    member.ondragstart = event => { draggedHost = { group: group.id, id }; event.dataTransfer.effectAllowed = 'move'; event.dataTransfer.setData('text/plain', id) }
                    member.ondragend = () => { draggedHost = null; members.querySelectorAll('.drop-before,.drop-after').forEach(node => node.classList.remove('drop-before','drop-after')) }
                    member.ondragover = event => { if (draggedHost?.group !== group.id || busy || changing) return; event.preventDefault(); event.dataTransfer.dropEffect = 'move'; const after = event.clientY > member.getBoundingClientRect().top + member.offsetHeight/2; member.classList.toggle('drop-after',after); member.classList.toggle('drop-before',!after) }
                    member.ondragleave = () => member.classList.remove('drop-before','drop-after')
                    member.ondrop = event => { if (draggedHost?.group !== group.id) return; event.preventDefault(); event.stopPropagation(); const from = group['host-ids'].indexOf(draggedHost.id), after = event.clientY > member.getBoundingClientRect().top + member.offsetHeight/2; draggedHost = null; member.ondragleave(); const insertion = index + (after ? 1 : 0); if (from >= 0) move(from,insertion-(from<insertion?1:0)) }
                    li.append(member); members.append(li)
                }); card.append(members)
            }
        }
        if (focusID) {
            const card = [...list.children].find(card => card.dataset.groupId === focusID)
            const next = card && [...card.querySelectorAll('[data-action]')].find(node => node.dataset.action === focusAction && !node.disabled)
            if (!document.querySelector('dialog[open]')) (next || panel.querySelector('[data-close]')).focus({ preventScroll: true })
        }
    }
    async function groups() {
        panel.hidden = false; container.classList.add('groups-open'); groupToggle.setAttribute('aria-expanded', 'true')
        if (changing) return
        const revision = ++groupRevision
        try { const data = await groupRequest(); if (revision !== groupRevision) return; groupData = data; groupLoaded = true; renderGroups() } catch (error) { if (revision === groupRevision) result.textContent = error.message }
    }
    // The toolbar toggles; saveGroup/groups() can always reveal the panel.
    groupToggle.onclick = () => panel.hidden ? groups() : hideGroups()
    return { sync, toggle, open, ids, groups, saveGroup, deleteSelected, selectAll() { if (!busy) { for (const h of hosts()) if (h['unique-id']) selected.add(h['unique-id']); sync() } }, clear() { if (!busy) { selected.clear(); sync() } } }
})()
