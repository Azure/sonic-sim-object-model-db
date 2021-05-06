FROM debian:buster

## Make apt-get non-interactive
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
 && apt-get install -y \
      vim \
      redis-server \
      supervisor \
      curl \
      bridge-utils \
      net-tools

COPY debs /debs
RUN dpkg -i /debs/libhiredis0.13_0.13.3-2_amd64.deb \
 && dpkg -i /debs/libhiredis-dev_0.13.3-2_amd64.deb \
 && dpkg -i /debs/libnl-3-200_3.2.27-2_amd64.deb \
 && dpkg -i /debs/libnl-genl-3-200_3.2.27-2_amd64.deb \
 && dpkg -i /debs/libnl-route-3-200_3.2.27-2_amd64.deb \
 && dpkg -i /debs/libswsscommon_1.0.0_amd64.deb \
 && dpkg -i /debs/libswsscommon-dev_1.0.0_amd64.deb \
 && dpkg -i /debs/sonic-rest-api_1.0.1_amd64.deb
RUN rm -fr /debs

COPY supervisor/supervisor.conf /etc/supervisor/conf.d/
COPY supervisor/rest_api.conf /etc/supervisor/conf.d/

RUN mkdir /usr/sbin/cert
RUN mkdir /usr/sbin/cert/client
RUN mkdir /usr/sbin/cert/server
COPY cert/client/* /usr/sbin/cert/client/
COPY cert/server/* /usr/sbin/cert/server/

RUN apt-get autoremove -y \
 && apt-get clean \
 && rm -fr /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENTRYPOINT ["/usr/bin/supervisord"]
