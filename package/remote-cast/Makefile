include $(TOPDIR)/rules.mk

PKG_NAME:=remote-cast
PKG_VERSION:=0.1.0
PKG_RELEASE:=1
PKG_LICENSE:=MIT
PKG_MAINTAINER:=remote-cast local build

PKG_BUILD_DIR:=$(BUILD_DIR)/$(PKG_NAME)-$(PKG_VERSION)

include $(INCLUDE_DIR)/package.mk

define Package/remote-cast
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Remote DLNA SSDP relay for EasyTier
  DEPENDS:=+libc
endef

define Package/remote-cast/description
  Application-level SSDP relay for exposing home DLNA/UPnP renderers
  to Android clients connected through EasyTier.
endef

define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	$(CP) ./src/* $(PKG_BUILD_DIR)/
endef

define Build/Compile
	cd $(PKG_BUILD_DIR) && \
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -trimpath -ldflags "-s -w" -o remote-castd .
endef

define Package/remote-cast/conffiles
/etc/config/remote-cast
endef

define Package/remote-cast/install
	$(INSTALL_DIR) $(1)/usr/sbin
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/remote-castd $(1)/usr/sbin/remote-castd
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) ./files/usr/bin/remote-castctl $(1)/usr/bin/remote-castctl
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/remote-cast $(1)/etc/init.d/remote-cast
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/etc/config/remote-cast $(1)/etc/config/remote-cast
endef

$(eval $(call BuildPackage,remote-cast))
