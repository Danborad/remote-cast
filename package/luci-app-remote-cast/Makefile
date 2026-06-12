include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-remote-cast
PKG_VERSION:=0.1.0
PKG_RELEASE:=1
PKG_LICENSE:=MIT

include $(INCLUDE_DIR)/package.mk

define Package/luci-app-remote-cast
  SECTION:=luci
  CATEGORY:=LuCI
  SUBMENU:=3. Applications
  TITLE:=LuCI support for remote-cast
  DEPENDS:=+luci-base +remote-cast
  PKGARCH:=all
endef

define Package/luci-app-remote-cast/description
  LuCI configuration and status pages for remote-cast.
endef

define Build/Compile
endef

define Package/luci-app-remote-cast/install
	$(INSTALL_DIR) $(1)/usr/share/luci/menu.d
	$(INSTALL_DATA) ./files/usr/share/luci/menu.d/luci-app-remote-cast.json $(1)/usr/share/luci/menu.d/luci-app-remote-cast.json
	$(INSTALL_DIR) $(1)/usr/share/rpcd/acl.d
	$(INSTALL_DATA) ./files/usr/share/rpcd/acl.d/luci-app-remote-cast.json $(1)/usr/share/rpcd/acl.d/luci-app-remote-cast.json
	$(INSTALL_DIR) $(1)/www/luci-static/resources/view/remote-cast
	$(INSTALL_DATA) ./files/www/luci-static/resources/view/remote-cast/config.js $(1)/www/luci-static/resources/view/remote-cast/config.js
	$(INSTALL_DATA) ./files/www/luci-static/resources/view/remote-cast/status.js $(1)/www/luci-static/resources/view/remote-cast/status.js
endef

$(eval $(call BuildPackage,luci-app-remote-cast))
