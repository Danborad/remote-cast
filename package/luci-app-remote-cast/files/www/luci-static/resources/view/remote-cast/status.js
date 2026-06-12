'use strict';
'require view';
'require fs';
'require poll';
'require ui';

function esc(s) {
	return String(s || '').replace(/[&<>"']/g, function(c) {
		return '&#' + c.charCodeAt(0) + ';';
	});
}

function renderRows(devices) {
	if (!devices || !devices.length) {
		return '<tr><td colspan="6"><em>' + _('尚未发现白名单内的电视设备') + '</em></td></tr>';
	}

	return devices.map(function(dev) {
		var warning = dev.warning ? '<br /><span class="label warning">' + esc(dev.warning) + '</span>' : '';
		return '<tr>' +
			'<td>' + esc(dev.ip) + warning + '</td>' +
			'<td>' + esc(dev.st) + '</td>' +
			'<td>' + esc(dev.usn) + '</td>' +
			'<td style="word-break:break-all">' + esc(dev.location) + '</td>' +
			'<td>' + esc(dev.server) + '</td>' +
			'<td>' + esc(dev.last_seen) + '</td>' +
		'</tr>';
	}).join('');
}

return view.extend({
	load: function() {
		return Promise.all([
			fs.exec('/usr/bin/remote-castctl', [ 'status' ]).catch(function(e) { return e; }),
			fs.read('/var/run/remote-cast/devices.json').catch(function() { return '[]'; })
		]);
	},

	render: function(data) {
		var status = data[0], raw = data[1], devices = [];
		var running = status && status.code === 0;

		try {
			devices = JSON.parse(raw || '[]');
		} catch (e) {
			devices = [];
		}

		var root = E('div', { 'class': 'cbi-map' }, [
			E('h2', _('远程投屏状态')),
			E('div', { 'class': 'cbi-section' }, [
				E('p', running ? _('服务正在运行') : _('服务未运行')),
				E('button', {
					'class': 'btn cbi-button cbi-button-apply',
					'click': ui.createHandlerFn(this, function() {
						return fs.exec('/usr/bin/remote-castctl', [ 'scan' ]).then(function() {
							ui.addNotification(null, E('p', _('已请求主动扫描，几秒后刷新状态。')));
						}).catch(function(e) {
							ui.addNotification(null, E('p', _('扫描失败：') + (e.message || e)), 'error');
						});
					})
				}, _('测试扫描'))
			]),
			E('div', { 'class': 'cbi-section' }, [
				E('h3', _('最近发现的电视')),
				E('table', { 'class': 'table' }, [
					E('tr', { 'class': 'tr table-titles' }, [
						E('th', _('IP')),
						E('th', _('ST')),
						E('th', _('USN')),
						E('th', _('LOCATION')),
						E('th', _('SERVER')),
						E('th', _('最后发现'))
					]),
					E('tbody', { 'id': 'remote-cast-devices' })
				])
			])
		]);

		root.querySelector('#remote-cast-devices').innerHTML = renderRows(devices);

		poll.add(function() {
			return fs.read('/var/run/remote-cast/devices.json').then(function(raw) {
				var devices = [];
				try {
					devices = JSON.parse(raw || '[]');
				} catch (e) {}
				var body = document.getElementById('remote-cast-devices');
				if (body)
					body.innerHTML = renderRows(devices);
			}).catch(function() {});
		}, 5);

		return root;
	}
});
