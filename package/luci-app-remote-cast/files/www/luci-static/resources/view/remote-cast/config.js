'use strict';
'require view';
'require form';
'require uci';

return view.extend({
	render: function() {
		var m, s, o;

		m = new form.Map('remote-cast', _('远程投屏'),
			_('通过 EasyTier 转发 DLNA/UPnP 的 SSDP 发现，让远端安卓手机能看到家里的电视。'));

		s = m.section(form.NamedSection, 'main', 'remote-cast', _('基础设置'));
		s.anonymous = true;

		o = s.option(form.Flag, 'enabled', _('启用'));
		o.default = '0';
		o.rmempty = false;

		o = s.option(form.Value, 'lan_iface', _('LAN 接口'));
		o.default = 'br-lan';
		o.placeholder = 'br-lan';
		o.rmempty = false;

		o = s.option(form.Value, 'easytier_iface', _('EasyTier 接口'));
		o.default = 'easytier0';
		o.placeholder = 'easytier0';
		o.rmempty = false;

		o = s.option(form.Value, 'phone_subnet', _('手机网段'));
		o.default = '10.0.0.0/8';
		o.placeholder = '10.0.0.0/8';
		o.rmempty = false;

		o = s.option(form.DynamicList, 'allowed_tv_ips', _('电视 IP 白名单'));
		o.datatype = 'ip4addr';
		o.placeholder = '192.168.1.100';
		o.rmempty = false;

		o = s.option(form.Flag, 'debug', _('调试日志'));
		o.default = '0';

		return m.render();
	}
});
