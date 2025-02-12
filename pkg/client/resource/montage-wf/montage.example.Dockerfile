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
    && apt-get clean

RUN curl -O http://montage.ipac.caltech.edu/download/Montage_v6.0.tar.gz && \
    tar xvfz Montage_v6.0.tar.gz && \
    rm Montage_v6.0.tar.gz && \
    cd Montage && \
    make

ENV PATH="/Montage/bin:${PATH}"
