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
    && wget https://github.com/bitxeno/PlumeImpactor/releases/download/v2.2.3-patch.4/plumesign-linux-${PKG_ARCH}.tar.gz \
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

# Download DeveloperDiskImages snapshot
RUN mkdir -p /keep \
    && cd /tmp \
    && wget -O DeveloperDiskImages.zip https://github.com/bitxeno/DeveloperDiskImages/archive/refs/heads/main.zip \
    && unzip DeveloperDiskImages.zip \
    && mv DeveloperDiskImages-main /keep/DeveloperDiskImages \
    && rm -rf /keep/DeveloperDiskImages/iOS_DDI \
    && rm -rf /keep/DeveloperDiskImages/.gitignore

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



# Install startup script
COPY ./doc/scripts/usbmuxd /etc/init.d/usbmuxd
RUN chmod +x /etc/init.d/usbmuxd
COPY ./doc/scripts/entrypoint.sh /entrypoint.sh
RUN sed -i "s/__APP_NAME__/${APP_NAME}/g" /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

EXPOSE 80
VOLUME /data
