# New-World-Auction-House-Crawler

Goal of this library is to have a process which grabs New World auction house data in the background while playing the game.

The process constists of the following steps:

1) Grabber: Grab screenshot periodically while playing
2) Parser: Check screenshots and detect if auction house UI is visible. If visible, parse displayed data with OCR and write to file
3) Uploader: Check files and write new data to a db of some kind