const API_URL = '/api/todos';

document.addEventListener('DOMContentLoaded', () => {
    fetchTodos();
    setupEventListeners();
});

let todos = [];
let todoIdToDelete = null;

async function fetchTodos() {
    try {
        const response = await fetch(API_URL);
        if (!response.ok) throw new Error('Failed to fetch todos');
        todos = await response.json();
        renderTodos();
    } catch (error) {
        console.error('Error:', error);
    }
}

function renderTodos() {
    const pendingList = document.getElementById('pending-list');
    const completedList = document.getElementById('completed-list');
    
    pendingList.innerHTML = '';
    completedList.innerHTML = '';

    const pending = todos.filter(t => !t.completed);
    pending.sort((a, b) => a.order - b.order);
    
    const completed = todos.filter(t => t.completed);

    document.getElementById('pending-count').textContent = pending.length;
    document.getElementById('completed-count').textContent = completed.length;

    // Render Pending
    pending.forEach(todo => {
        pendingList.appendChild(createTodoElement(todo));
    });

    // Render Completed (Grouped)
    renderCompletedGroups(completed, completedList);
}

function renderCompletedGroups(completedTodos, container) {
    if (completedTodos.length === 0) return;

    // Groups: Today, This Week, This Month, Older
    const groups = {
        today: [],
        week: [],
        month: [],
        older: []
    };

    const now = new Date();
    const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    
    // Calculate start of week (Monday)
    const day = now.getDay() || 7; // Get current day number, converting Sun(0) to 7
    if (day !== 1) now.setHours(-24 * (day - 1)); // Set to Monday
    const weekStart = new Date(now.getFullYear(), now.getMonth(), now.getDate());

    const monthStart = new Date(new Date().getFullYear(), new Date().getMonth(), 1);

    completedTodos.forEach(todo => {
        // Handle completed_at being potentially empty/null (backward compatibility)
        // If empty, treat as "Older" or maybe "Today" if we just did it? 
        // Backend handles filling it on update. Old data might be empty.
        // Let's treat empty as "Older" or "Unknown"
        if (!todo.completed_at || todo.completed_at === "0001-01-01T00:00:00Z") {
            groups.older.push(todo);
            return;
        }

        const date = new Date(todo.completed_at);
        
        if (date >= todayStart) {
            groups.today.push(todo);
        } else if (date >= weekStart) {
            groups.week.push(todo);
        } else if (date >= monthStart) {
            groups.month.push(todo);
        } else {
            groups.older.push(todo);
        }
    });

    // Sort within groups by completion time desc
    const sortFn = (a, b) => new Date(b.completed_at) - new Date(a.completed_at);
    
    // Render Groups
    renderGroup(container, "Today", groups.today.sort(sortFn));
    renderGroup(container, "This Week", groups.week.sort(sortFn));
    renderGroup(container, "This Month", groups.month.sort(sortFn));
    renderGroup(container, "Older", groups.older.sort(sortFn));
}

function renderGroup(container, title, items) {
    if (items.length === 0) return;

    const groupDiv = document.createElement('div');
    groupDiv.className = 'completed-group';
    
    const titleEl = document.createElement('div');
    titleEl.className = 'group-title';
    titleEl.textContent = title;
    groupDiv.appendChild(titleEl);

    const ul = document.createElement('ul');
    ul.className = 'todo-list';
    
    items.forEach(todo => {
        ul.appendChild(createTodoElement(todo));
    });
    
    groupDiv.appendChild(ul);
    container.appendChild(groupDiv);
}

function createTodoElement(todo) {
    const li = document.createElement('li');
    li.className = `todo-item ${todo.completed ? 'completed' : ''}`;
    li.dataset.id = todo.id;
    
    if (!todo.completed) {
        li.draggable = true;
        li.addEventListener('dragstart', handleDragStart);
        li.addEventListener('dragend', handleDragEnd);
    }

    const createdDate = new Date(todo.created_at).toLocaleDateString();
    // Only show completed time if completed
    let metaText = `Created: ${createdDate}`;
    if (todo.completed && todo.completed_at && todo.completed_at !== "0001-01-01T00:00:00Z") {
        const completedDate = new Date(todo.completed_at).toLocaleString();
        metaText += ` • Done: ${completedDate}`;
    }

    li.innerHTML = `
        <div class="checkbox"></div>
        <div class="todo-content-wrapper">
            <div class="content">${escapeHtml(todo.content)}</div>
            <div class="todo-meta">${metaText}</div>
        </div>
        <button class="delete-btn">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
        </button>
    `;

    li.querySelector('.content').addEventListener('click', (e) => {
        e.stopPropagation();
        showEditModal(todo);
    });

    li.querySelector('.checkbox').addEventListener('click', (e) => {
        e.stopPropagation(); // Prevent drag triggering if any
        toggleTodo(todo);
    });
    li.querySelector('.delete-btn').addEventListener('click', (e) => {
        e.stopPropagation();
        deleteTodo(todo.id);
    });

    return li;
}

function setupEventListeners() {
    const addBtn = document.getElementById('add-btn');
    const input = document.getElementById('new-todo');

    addBtn.addEventListener('click', () => handleAdd());
    input.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') handleAdd();
    });

    const pendingList = document.getElementById('pending-list');
    pendingList.addEventListener('dragover', handleDragOver);

    // Modal Event Listeners
    const modal = document.getElementById('confirm-modal');
    const cancelBtn = document.getElementById('cancel-delete');
    const confirmBtn = document.getElementById('confirm-delete');

    cancelBtn.addEventListener('click', hideDeleteModal);
    confirmBtn.addEventListener('click', confirmDelete);

    // Edit Modal Event Listeners
    const editModal = document.getElementById('edit-modal');
    const cancelEditBtn = document.getElementById('cancel-edit');
    const saveEditBtn = document.getElementById('save-edit');
    const editInput = document.getElementById('edit-input');

    cancelEditBtn.addEventListener('click', hideEditModal);
    saveEditBtn.addEventListener('click', saveEdit);
    editInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') saveEdit();
    });
    
    // Close modal if clicked outside
    window.addEventListener('click', (e) => {
        if (e.target.classList.contains('modal')) {
            e.target.classList.remove('show');
            // Reset delete state if it was the delete modal
            if (e.target.id === 'confirm-modal') {
                todoIdToDelete = null;
            }
            // Reset edit state if it was the edit modal
            if (e.target.id === 'edit-modal') {
                currentEditTodo = null;
            }
        }
    });
}

// Edit Logic
let currentEditTodo = null;

function showEditModal(todo) {
    currentEditTodo = todo;
    const modal = document.getElementById('edit-modal');
    const input = document.getElementById('edit-input');
    input.value = todo.content;
    modal.classList.add('show');
    input.focus();
}

function hideEditModal() {
    currentEditTodo = null;
    const modal = document.getElementById('edit-modal');
    modal.classList.remove('show');
}

async function saveEdit() {
    if (!currentEditTodo) return;
    
    const input = document.getElementById('edit-input');
    const newContent = input.value.trim();
    
    if (!newContent || newContent === currentEditTodo.content) {
        hideEditModal();
        return;
    }

    const updatedTodo = { ...currentEditTodo, content: newContent };
    
    // Optimistic update
    const index = todos.findIndex(t => t.id === currentEditTodo.id);
    if (index !== -1) {
        todos[index] = updatedTodo;
        renderTodos();
    }
    
    hideEditModal();

    try {
        const response = await fetch(`${API_URL}/${updatedTodo.id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(updatedTodo)
        });

        if (!response.ok) throw new Error('Failed to update todo');
    } catch (error) {
        console.error('Error:', error);
        await fetchTodos();
    }
}

function showDeleteModal(id) {
    todoIdToDelete = id;
    const modal = document.getElementById('confirm-modal');
    modal.classList.add('show');
}

function hideDeleteModal() {
    todoIdToDelete = null;
    const modal = document.getElementById('confirm-modal');
    modal.classList.remove('show');
}

// Summary Functions
let currentSummaryPeriod = null;
let currentSummaryText = '';

function getSummary(period) {
    currentSummaryPeriod = period;
    currentSummaryText = '';
    const modal = document.getElementById('summary-modal');
    const contentDiv = document.getElementById('summary-content');
    const actionsDiv = modal.querySelector('.modal-actions');
    const title = modal.querySelector('h3');

    title.textContent = '✨ AI 总结确认';
    contentDiv.innerHTML = `
        <div style="text-align: center; padding: 10px 0;">
            <p style="margin-bottom: 10px; font-size: 1.1em;">即将使用 <strong>豆包大模型</strong> 为您总结 <strong>${period}</strong> 的任务完成情况。</p>
            <p style="color: rgba(255,255,255,0.5); font-size: 0.9em;">生成过程可能需要几秒钟，请耐心等待。</p>
        </div>
    `;
    
    actionsDiv.innerHTML = `
        <button class="btn btn-secondary" onclick="closeSummaryModal()">取消</button>
        <button class="btn btn-primary" onclick="startSummaryGeneration()">确认开始</button>
    `;

    modal.classList.add('show');
}

async function startSummaryGeneration() {
    if (!currentSummaryPeriod) return;

    const modal = document.getElementById('summary-modal');
    const contentDiv = document.getElementById('summary-content');
    const actionsDiv = modal.querySelector('.modal-actions');
    const title = modal.querySelector('h3');

    title.textContent = 'AI Summary';
    contentDiv.innerHTML = '<div style="text-align: center; padding: 20px;">正在生成总结... 🤖<br><span style="font-size:0.8em; color:rgba(255,255,255,0.5);">(Thinking...)</span></div>';
    
    // Show disabled button while processing
    actionsDiv.innerHTML = `
        <button class="btn btn-secondary" style="opacity: 0.5; cursor: not-allowed;">生成中...</button>
    `;

    try {
        const response = await fetch(`/api/summary?period=${currentSummaryPeriod}`);
        const data = await response.json();
        
        if (data.summary) {
            currentSummaryText = data.summary;
            contentDiv.innerHTML = marked.parse(data.summary);
        } else {
            currentSummaryText = '';
            contentDiv.textContent = '未能生成总结，请重试。';
        }
    } catch (error) {
        console.error('Error:', error);
        currentSummaryText = '';
        contentDiv.textContent = '获取总结失败，请检查网络连接。';
    } finally {
        actionsDiv.innerHTML = `
            <button class="btn btn-secondary" onclick="copySummaryToClipboard()">复制总结</button>
            <button class="btn btn-primary" onclick="closeSummaryModal()">关闭</button>
        `;
    }
}

function copySummaryToClipboard() {
    if (!currentSummaryText) return;
    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(currentSummaryText).catch(() => {
            const textarea = document.createElement('textarea');
            textarea.value = currentSummaryText;
            textarea.style.position = 'fixed';
            textarea.style.opacity = '0';
            document.body.appendChild(textarea);
            textarea.focus();
            textarea.select();
            try {
                document.execCommand('copy');
            } finally {
                document.body.removeChild(textarea);
            }
        });
    } else {
        const textarea = document.createElement('textarea');
        textarea.value = currentSummaryText;
        textarea.style.position = 'fixed';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.focus();
        textarea.select();
        try {
            document.execCommand('copy');
        } finally {
            document.body.removeChild(textarea);
        }
    }
}

function closeSummaryModal() {
    const modal = document.getElementById('summary-modal');
    modal.classList.remove('show');
}

async function confirmDelete() {
    if (!todoIdToDelete) return;
    
    const id = todoIdToDelete;
    hideDeleteModal();

    try {
        todos = todos.filter(t => t.id !== id);
        renderTodos();

        const response = await fetch(`${API_URL}/${id}`, {
            method: 'DELETE'
        });

        if (!response.ok) throw new Error('Failed to delete todo');
    } catch (error) {
        console.error('Error:', error);
        await fetchTodos();
    }
}

async function handleAdd() {
    const input = document.getElementById('new-todo');
    const content = input.value.trim();
    if (!content) return;

    try {
        const response = await fetch(API_URL, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                content: content,
                completed: false,
                order: todos.length > 0 ? Math.max(...todos.map(t => t.order)) + 1 : 0
            })
        });

        if (!response.ok) throw new Error('Failed to add todo');
        
        input.value = '';
        await fetchTodos();
    } catch (error) {
        console.error('Error:', error);
    }
}

async function toggleTodo(todo) {
    try {
        // For optimistic update, we need to guess the new state.
        // But for time grouping, we need the server time or generate local time.
        // Let's just generate local time for optimistic update.
        const now = new Date().toISOString();
        const updatedTodo = { 
            ...todo, 
            completed: !todo.completed,
            completed_at: !todo.completed ? now : null 
        };
        
        // Optimistic update
        const index = todos.findIndex(t => t.id === todo.id);
        todos[index] = updatedTodo;
        renderTodos();

        const response = await fetch(`${API_URL}/${todo.id}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(updatedTodo)
        });

        if (!response.ok) {
            // Revert
            todos[index] = todo;
            renderTodos();
            throw new Error('Failed to update todo');
        } else {
            // Fetch real state from server to ensure times are synced correctly
            // (Server might have slightly different time or formatted it differently)
            await fetchTodos();
        }
    } catch (error) {
        console.error('Error:', error);
    }
}

async function deleteTodo(id) {
    showDeleteModal(id);
}

// Drag and Drop Logic
let draggedItem = null;

function handleDragStart(e) {
    draggedItem = this;
    setTimeout(() => this.classList.add('dragging'), 0);
    e.dataTransfer.effectAllowed = 'move';
}

function handleDragEnd(e) {
    this.classList.remove('dragging');
    draggedItem = null;
    saveNewOrder();
}

function handleDragOver(e) {
    e.preventDefault();
    const pendingList = document.getElementById('pending-list');
    const afterElement = getDragAfterElement(pendingList, e.clientY);
    const draggable = document.querySelector('.dragging');
    if (!draggable) return;

    if (afterElement == null) {
        pendingList.appendChild(draggable);
    } else {
        pendingList.insertBefore(draggable, afterElement);
    }
}

function getDragAfterElement(container, y) {
    const draggableElements = [...container.querySelectorAll('.todo-item:not(.dragging)')];

    return draggableElements.reduce((closest, child) => {
        const box = child.getBoundingClientRect();
        const offset = y - box.top - box.height / 2;
        if (offset < 0 && offset > closest.offset) {
            return { offset: offset, element: child };
        } else {
            return closest;
        }
    }, { offset: Number.NEGATIVE_INFINITY }).element;
}

async function saveNewOrder() {
    const pendingList = document.getElementById('pending-list');
    const newOrderIds = [...pendingList.querySelectorAll('.todo-item')].map(item => item.dataset.id);
    
    try {
        await fetch('/api/reorder', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(newOrderIds)
        });
    } catch (error) {
        console.error('Error saving order:', error);
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}
