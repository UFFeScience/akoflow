FROM debian:buster-slim

RUN apt update && apt install -y \
    build-essential \
    curl \
    libcfitsio-dev \
    libgsl-dev \
    libjpeg-dev \
    libpng-dev \
    libtiff-dev \
    libx11-dev \
    libxext-dev \
    libxft-dev \
    libxmu-dev \
    libxpm-dev \
    libxt-dev \
    wget \
    unzip \
    && apt-get clean

RUN curl -O http://montage.ipac.caltech.edu/download/Montage_v6.0.tar.gz && \
    tar xvfz Montage_v6.0.tar.gz && \
    rm Montage_v6.0.tar.gz && \
    cd Montage && \
    make

ENV PATH="/Montage/bin:${PATH}"

WORKDIR /data

#
RUN curl https://ovvesley.com/uff.scik8sflow/data-out-0-50d.zip -o data-out-0-50d.zip && \
    unzip data-out-0-50d.zip && \
    cp -R data-out-0-50d/* . && \
    rm -rf data-out-0-50d.zip data-out-0-50d

