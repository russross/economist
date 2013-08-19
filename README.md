Economist
=========

A script to prepare the audio edition of The Economist for an MP3 player.

I listed to the audio edition of "The Economist" while I drive to and from work.
This Go script takes care of loading the latest edition onto an SD card for my
in-car MP3 player.

* It assumes that the SD card is mounted at /media/$USER/economist
* It assumes that the latest edition is in $HOME/Downloads
* It deletes the section heading files and the letters (I read these online sometimes)
* It organizes the files into folders, one per section from the newspaper
* It re-encodes the MP3s to increase the volume. They were too quiet to use in my car.
* It renames the files to remove spaces and most punctuation.

This is mainly for my own personal use. I doubt anyone else will find it useful, but would
love to hear if anyone else uses it.
