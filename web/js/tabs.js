// Connection views can register their own DOM and cleanup without rebuilding
// hidden panels, so switching tabs preserves their state.
const workspaceTabs = (() => {
    const list = document.querySelector('#workspace-tabs')
    const panels = document.querySelector('#workspace-panels')
    const hostsButton = document.querySelector('#tab-hosts')
    const tabs = new Map([['hosts', {
        button: hostsButton,
        wrapper: hostsButton.parentElement,
        panel: document.querySelector('#hosts-data-container'),
    }]])
    let active = 'hosts'
    let sequence = 0
    let exiting = false

    // Capture before inputs/file lists and suppress the browser close shortcut.
    document.addEventListener('keydown', async event => {
        if (!event.ctrlKey || event.altKey || event.metaKey || event.shiftKey || event.code !== 'KeyW') return
        event.preventDefault(); event.stopImmediatePropagation()
        if (event.repeat || exiting || document.querySelector('dialog[open]')) return
        if (active !== 'hosts') { await close(active); return }
        exiting = true
        try {
            if (!await appDialogs.confirm('Exit SSH Manager? Any active file transfers will be interrupted.', { title: 'Exit SSH Manager', confirmText: 'Exit', danger: true })) return
            const response = await fetch('/application/exit', { method: 'POST' })
            if (!response.ok) throw new Error(`Exit failed (${response.status})`)
        } catch (error) { await appDialogs.alert(error.message) }
        finally { exiting = false }
    }, true)

    function select(id, focus = false) {
        if (!tabs.has(id)) return
        active = id
        for (const [key, tab] of tabs) {
            const selected = key === id
            tab.button.setAttribute('aria-selected', String(selected))
            tab.button.tabIndex = 0
            tab.panel.hidden = !selected
        }
        document.querySelectorAll('[data-host-tools] button, [data-host-tool]').forEach(button => {
            button.disabled = id !== 'hosts'
        })
        const button = tabs.get(id).button
        button.scrollIntoView({ block: 'nearest', inline: 'nearest' })
        if (focus) button.focus()
    }

    function open({ id, title, content, onClose }) {
        if (tabs.has(id)) { select(id); return }
        if (!id || !(content instanceof HTMLElement)) throw new Error('A tab needs an ID and content element')
        const token = `connection-${++sequence}`
        const wrapper = document.createElement('div')
        wrapper.className = 'workspace-tab'
        const button = document.createElement('button')
        button.className = 'workspace-tab-button'
        button.type = 'button'
        button.id = `tab-${token}`
        button.setAttribute('role', 'tab')
        button.setAttribute('aria-controls', `panel-${token}`)
        button.textContent = title
        button.title = title
        button.addEventListener('click', () => select(id))
        const closeButton = document.createElement('button')
        closeButton.className = 'workspace-tab-close'
        closeButton.type = 'button'
        closeButton.tabIndex = -1
        closeButton.textContent = '×'
        closeButton.title = 'Close tab (Ctrl+W on active tab; Delete on focused tab)'
        closeButton.setAttribute('aria-label', `Close ${title}`)
        closeButton.addEventListener('click', () => close(id))
        content.id = `panel-${token}`
        content.setAttribute('role', 'tabpanel')
        content.setAttribute('aria-labelledby', button.id)
        wrapper.append(button, closeButton)
        list.append(wrapper)
        panels.append(content)
        tabs.set(id, { button, wrapper, panel: content, onClose })
        select(id)
    }

    async function close(id) {
        if (id === 'hosts') return
        const tab = tabs.get(id)
        if (!tab || tab.closing) return
        tab.closing = true
        try {
            if (tab.onClose && await tab.onClose() === false) return
            const keys = [...tabs.keys()]
            const previous = keys[Math.max(0, keys.indexOf(id) - 1)]
            const wasActive = active === id
            tab.wrapper.remove()
            tab.panel.remove()
            tabs.delete(id)
            if (wasActive) select(previous, true)
        } finally {
            tab.closing = false
        }
    }

    hostsButton.addEventListener('click', () => select('hosts'))
    function focusPanel() {
        if (active === 'hosts') {
            const row = restoreListNavigation()
            if (row) focusListRow(row)
            else { const panel = tabs.get(active).panel; panel.tabIndex = -1; panel.focus() }
        } else {
            const panel = tabs.get(active).panel
            const target = panel.querySelector('.file-list .file-selected')
                || panel.querySelector('.file-list [data-file-path]') || panel.querySelector('.file-list')
            target?.focus()
        }
    }
    document.querySelector('.workspace-header').addEventListener('keydown', event => {
        if (event.ctrlKey || event.altKey || event.metaKey || event.shiftKey || event.isComposing) return
        const vertical = event.key === 'ArrowUp' || event.key === 'ArrowDown'
        const tool = event.target.closest('.workspace-tools button')
        if (!vertical && !(tool && ['ArrowLeft', 'ArrowRight'].includes(event.key))) return
        if (!tool && !event.target.matches('[role="tab"]')) return
        event.preventDefault(); event.stopPropagation()
        const tab = [...tabs.entries()].find(([, tab]) => tab.button === event.target)
        if (tab) select(tab[0])
        focusPanel()
    })
    list.addEventListener('keydown', event => {
        if (!event.target.matches('[role="tab"]') || event.ctrlKey || event.altKey || event.metaKey) return
        const keys = [...tabs.keys()]
        let index = keys.findIndex(key => tabs.get(key).button === event.target)
        switch (event.key) {
            case 'ArrowLeft': index = (index - 1 + keys.length) % keys.length; break
            case 'ArrowRight': index = (index + 1) % keys.length; break
            case 'Home': index = 0; break
            case 'End': index = keys.length - 1; break
            case 'Delete': event.preventDefault(); close(keys[index]); return
            default: return
        }
        event.preventDefault()
        if (keys.length === 1 && ['ArrowLeft', 'ArrowRight'].includes(event.key)) { focusPanel(); return }
        select(keys[index], true)
    })
    return { open, select, close }
})()
