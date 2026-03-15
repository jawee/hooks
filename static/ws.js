// WebSocket client for listener page
async function startWebSocket(uuid) {
    // Try to refresh JWT before opening WebSocket
    try {
        await fetch('/refresh', { method: 'POST', credentials: 'same-origin' });
    } catch (e) {
        // Ignore errors; if refresh fails, wsHandler will reject
    }
    var wsProto = window.location.protocol === "https:" ? "wss://" : "ws://";
    var ws = new WebSocket(wsProto + window.location.host + "/ws/" + uuid);
    ws.onmessage = function(event) {
        var ul = document.getElementById("requests");
        if (ul) {
            // Remove 'No requests received yet.' if present
            var noReqMsg = document.querySelector("#requests-empty");
            if (noReqMsg) noReqMsg.remove();

            var req = JSON.parse(event.data);
            var idx = ul.children.length;
            var li = document.createElement("li");
            li.className = "border rounded-lg bg-white shadow p-4";
            // Time and toggle
            var topDiv = document.createElement("div");
            topDiv.className = "flex items-center justify-between";
            var timeSpan = document.createElement("span");
            timeSpan.className = "text-sm text-gray-500";
            timeSpan.innerHTML = '<b>Time:</b> ' + req.Timestamp;
            var btn = document.createElement("button");
            btn.className = "text-blue-600 underline text-sm focus:outline-none";
            btn.textContent = "Show Headers";
            btn.onclick = function() {
                var el = document.getElementById('headers-' + idx);
                if (el.classList.contains('hidden')) {
                    el.classList.remove('hidden');
                } else {
                    el.classList.add('hidden');
                }
            };
            topDiv.appendChild(timeSpan);
            topDiv.appendChild(btn);
            li.appendChild(topDiv);
            // Headers
            var headersDiv = document.createElement("div");
            headersDiv.id = 'headers-' + idx;
            headersDiv.className = 'hidden mt-2 bg-gray-50 border rounded p-2 text-xs';
            var headersPre = document.createElement("pre");
            headersPre.className = "whitespace-pre-wrap";
            var headerLines = [];
            for (var k in req.Headers) {
                headerLines.push(k + ': ' + req.Headers[k].join(", "));
            }
            headersPre.textContent = headerLines.join("\n");
            headersDiv.appendChild(headersPre);
            li.appendChild(headersDiv);
            // Body
            var bodyDiv = document.createElement("div");
            bodyDiv.className = "mt-2";
            var bodyLabel = document.createElement("div");
            bodyLabel.className = "font-semibold text-gray-700";
            bodyLabel.textContent = "Body:";
            var pre = document.createElement("pre");
            pre.className = "bg-gray-100 rounded p-2 mt-1 text-sm whitespace-pre-wrap";
            pre.textContent = req.Body;
            bodyDiv.appendChild(bodyLabel);
            bodyDiv.appendChild(pre);
            li.appendChild(bodyDiv);
            ul.insertBefore(li, ul.firstChild);
        }
    };
    ws.onclose = function() {
        setTimeout(function() { startWebSocket(uuid); }, 2000);
    };
}
