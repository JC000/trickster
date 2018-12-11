%define AutoReqProv: no
%define __os_install_post %{nil}

Name:       trickster
Version:    %{_version}
Release:    %{_release}
Summary:    Dashboard Accelerator for Prometheus

Vendor:     Comcast
Packager:   Comcast
Group:      System Environment/Daemons
License:    ASL 2.0
URL:        https://github.com/Comcast/trickster

Prefix:     /opt
BuildRoot:  %{_tmppath}/%{name}

%description
Trickster is a reverse proxy cache for the Prometheus HTTP APIv1 that
dramatically accelerates dashboard rendering times for any series queried
from Prometheus.

%install
echo rm -rf %{buildroot}
%{__install} -d %{buildroot}%{_bindir}
%{__install} -d %{buildroot}%{_initddir}
%{__install} -d %{buildroot}%{_sysconfdir}/%{name}
%{__install} -d %{buildroot}%{_localstatedir}/log/%{name}
%{__install} -d %{buildroot}%{_localstatedir}/run/%{name}

%{__install} -p $RPM_SOURCE_DIR/%{name} %{buildroot}%{_bindir}
%{__install} -p $RPM_SOURCE_DIR/%{name}.conf %{buildroot}%{_sysconfdir}/%{name}/%{name}.conf

%files
%defattr(644, root, root, 755)

%attr(755, root, root) %{_bindir}/%{name}

%dir %{_sysconfdir}/%{name}
%config %attr(644, %{name}, %{name}) %{_sysconfdir}/%{name}/%{name}.conf

%dir %{_localstatedir}/log/%{name}
%dir %{_localstatedir}/run/%{name}

%pre
id %{name} >/dev/null 2>&1
if [ $? != 0 ]; then
    /usr/sbin/groupadd -r %{name} >/dev/null 2>&1
    /usr/sbin/useradd -d /var/run/%{name} -r -g %{name} %{name} >/dev/null 2>&1
fi

%post
if [ $1 = 1 ]; then
    echo /sbin/chkconfig --add %{name}
fi

%preun
if [ -e /etc/init.d/%{name} ]; then
    echo /sbin/service %{name} stop > /dev/null 2>&1
    true
fi

# If not an upgrade, then delete
if [ $1 = 0 ]; then
    echo /sbin/chkconfig --del %{name} > /dev/null 2>&1
    true
fi

%postun
# Do not remove anything if this is not an uninstall
if [ $1 = 0 ]; then
    /usr/sbin/userdel -r %{name} >/dev/null 2>&1
    /usr/sbin/groupdel %{name} >/dev/null 2>&1
    # Ignore errors from above
    true
fi

%changelog
