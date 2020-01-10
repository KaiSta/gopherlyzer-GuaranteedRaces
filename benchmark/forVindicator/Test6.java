package test;

public class Test6 extends Thread {
  static int x;

  static void sleepSec(float sec) {
    try {
	Thread.sleep((long)(sec*1000));
    } catch (Exception e) {
       throw new RuntimeException(e);
    }
  }


  @Override
  public void run() {
   	//T1
	x = 1;
	x = 2;
	x = 3;
	x = 4;
	x = 5;
	x = 6;
	x = 7;
	x = 8;
	x = 9;
	x = 10;
	x = 11;
	x = 12;
	x = 13;
	x = 14;
	x = 15;
	x = 16;
	x = 17;
	x = 18;
	x = 19;
	x = 20;
	x = 21;
	x = 22;
	x = 23;
	x = 24;
	x = 25;
	x = 26;
	x = 27;
	x = 28;
	x = 29;
	x = 30;
	System.out.println("1w(x)");
  } 

   public static class T2 extends Thread implements Runnable {
		public void run() {
			sleepSec(5);
			x = 31;
			System.out.println("2w(x)");
		}
   }
   


   public static void main(String args[]) throws Exception {
	final Test6 thread1 = new Test6();
    final T2 thread2 = new T2();

	thread1.start();
	thread2.start();

	thread1.join();
	thread2.join();

   }
}
