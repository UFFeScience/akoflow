FROM python:3.12

RUN apt-get update && apt-get install -y clustalw

RUN git clone https://github.com/ovvesley/NMFSt.P.git

WORKDIR /NMFSt.P

RUN python -m pip install  ipython nbconvert nbformat

RUN python -m pip install -r requirements.txt

