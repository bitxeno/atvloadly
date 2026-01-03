FROM ubuntu:22.04
ARG APP_NAME
ARG VERSION
ARG BUILDDATE
ARG COMMIT
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
RUN echo "I'm building for $TARGETPLATFORM"

# Install dependencies
RUN apt-get update && apt-get -y install \
    wget libavahi-compat-libdnssd-dev curl

RUN case ${TARGETARCH} in \
         "amd64")  PKG_ARCH=x86_64  ;; \
         "arm64")  PKG_ARCH=aarch64  ;; \
    esac \
    && cd /tmp \
    && wget https://github.com/bitxeno/usbmuxd2/releases/download/v0.0.4/usbmuxd2-ubuntu-${PKG_ARCH}.tar.gz \
    && tar zxf usbmuxd2-ubuntu-${PKG_ARCH}.tar.gz \
    && dpkg -i ./libusb_1.0.26-1_${PKG_ARCH}.deb \
    && dpkg -i ./libgeneral_1.0.0-1_${PKG_ARCH}.deb \
    && dpkg -i ./libplist_2.6.0-1_${PKG_ARCH}.deb \
    && dpkg -i ./libtatsu_1.0.3-1_${PKG_ARCH}.deb \
    && dpkg -i ./libimobiledevice-glue_1.3.0-1_${PKG_ARCH}.deb \
    && dpkg -i ./libusbmuxd_2.3.0-1_${PKG_ARCH}.deb \
    && dpkg -i ./libimobiledevice_1.3.1-1_${PKG_ARCH}.deb \
    && dpkg -i ./usbmuxd2_1.0.0-1_${PKG_ARCH}.deb

# Install PlumeImpactor
RUN case ${TARGETARCH} in \
         "amd64")  PKG_ARCH=x86_64  ;; \
         "arm64")  PKG_ARCH=aarch64  ;; \
    esac \
    && cd /tmp \
    && wget https://github.com/bitxeno/PlumeImpactor/download/v1.4.0-alpha.1/plumesign-linux-${PKG_ARCH} \
    && mv plumesign-linux-${PKG_ARCH} /usr/bin/plumesign \
    && chmod +x /usr/bin/plumesign

# Install tzdata to support timezone updates.
RUN DEBIAN_FRONTEND=noninteractive apt-get -y install tzdata

# Clear apt cache and temporary data to reduce image size.
RUN apt-get clean
RUN cd /tmp && rm ./*.deb && rm ./*.tar.gz

# The add command will automatically decompress the file.
RUN mkdir -p /doc
COPY ./doc/config.yaml.example /doc/config.yaml
COPY ./build/${APP_NAME}-${TARGETOS}-${TARGETARCH} /usr/bin/${APP_NAME}
RUN chmod +x /usr/bin/${APP_NAME}

# The lockdown records have been moved to /data.
RUN rm -rf /var/lib/lockdown && mkdir -p /data/lockdown && ln -s /data/lockdown /var/lib/lockdown



# Generate startup script
COPY ./doc/scripts/usbmuxd /etc/init.d/usbmuxd
RUN chmod +x /etc/init.d/usbmuxd
RUN printf '#!/bin/sh \n\n\

mkdir -p /data/lockdown \n\
mkdir -p /data/PlumeImpactor \n\
ln -s /data/PlumeImpactor ~/.config/PlumeImpactor \n\

if [ ! -f "/data/config.yaml" ]; then  \n\
    cp /doc/config.yaml /data/config.yaml \n\
fi  \n\

/etc/init.d/usbmuxd start \n\

/usr/bin/%s server -p ${SERVICE_PORT:-80} -c /data/config.yaml  \n\
\n\
' ${APP_NAME} >> /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

EXPOSE 80
VOLUME /data
