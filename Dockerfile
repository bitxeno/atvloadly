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
    wget unzip libavahi-compat-libdnssd-dev curl

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
    && wget https://github.com/bitxeno/PlumeImpactor/releases/download/v2.0.0-patch.2/plumesign-linux-${PKG_ARCH}.tar.gz \
    && tar zxf plumesign-linux-${PKG_ARCH}.tar.gz \
    && mv plumesign-linux-${PKG_ARCH} /usr/bin/plumesign \
    && chmod +x /usr/bin/plumesign

# Download anisette dependency library
RUN case ${TARGETARCH} in \
         "amd64")  PKG_ARCH=x86_64  ;; \
         "arm64")  PKG_ARCH=arm64-v8a  ;; \
    esac \
    && mkdir -p /keep \
    && cd /keep \
    && wget https://apps.mzstatic.com/content/android-apple-music-apk/applemusic.apk \
    && unzip applemusic.apk lib/${PKG_ARCH}/libstoreservicescore.so lib/${PKG_ARCH}/libCoreADI.so \
    && rm applemusic.apk

# Install tzdata to support timezone updates.
RUN DEBIAN_FRONTEND=noninteractive apt-get -y install tzdata

# Clear apt cache and temporary data to reduce image size.
RUN apt-get clean
RUN cd /tmp && rm -rf ./*.deb && rm -rf ./*.tar.gz && rm -rf ./*.zip && rm -rf ./*.apk

# The add command will automatically decompress the file.
RUN mkdir -p /keep
COPY ./doc/config.yaml.example /keep/config.yaml
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
mkdir -p $HOME/.config \n\
[ ! -e "$HOME/.config/PlumeImpactor" ] && ln -s /data/PlumeImpactor $HOME/.config/PlumeImpactor \n\

if [ -d "/keep/lib" ]; then  \n\
    rm -rf /data/PlumeImpactor/lib \n\
    cp -rf /keep/lib /data/PlumeImpactor/lib \n\
    rm -rf /keep/lib \n\
fi  \n\

if [ ! -f "/data/config.yaml" ]; then  \n\
    cp /keep/config.yaml /data/config.yaml \n\
fi  \n\

/etc/init.d/usbmuxd start \n\

/usr/bin/%s server -p ${SERVICE_PORT:-80} -c /data/config.yaml  \n\
\n\
' ${APP_NAME} >> /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

EXPOSE 80
VOLUME /data
